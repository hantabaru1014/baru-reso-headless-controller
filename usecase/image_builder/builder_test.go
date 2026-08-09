package image_builder

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/hantabaru1014/baru-reso-headless-controller/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLogCapture(t *testing.T) {
	t.Parallel()

	t.Run("上限以内ならそのまま保持する", func(t *testing.T) {
		t.Parallel()

		c := newLogCapture(100)

		n, err := c.Write([]byte("hello "))
		require.NoError(t, err)
		assert.Equal(t, 6, n, "Write は受け取ったバイト数をそのまま返す (io.MultiWriter が短絡しないため)")

		_, _ = c.Write([]byte("world"))

		assert.Equal(t, "hello world", c.String())
	})

	t.Run("上限を超えたら先頭を捨てて末尾を残す", func(t *testing.T) {
		t.Parallel()

		c := newLogCapture(10)

		// 失敗の原因はログ末尾に出るので、残すべきは末尾側.
		_, _ = c.Write([]byte("0123456789ABCDE"))

		got := c.String()
		assert.True(t, strings.HasPrefix(got, logTruncationNotice), "切り詰めた旨を先頭に付ける")
		assert.Equal(t, "56789ABCDE", strings.TrimPrefix(got, logTruncationNotice))
	})

	t.Run("行途中で切れたら欠けた先頭行を捨てる", func(t *testing.T) {
		t.Parallel()

		c := newLogCapture(10)

		// 切断位置は行途中どころか UTF-8 の途中にもなりうるので、欠けた行ごと落とす.
		_, _ = c.Write([]byte("secret-xyz-abc\nkept\n"))

		assert.Equal(t, "kept\n", strings.TrimPrefix(c.String(), logTruncationNotice))
	})

	t.Run("行頭で切れたらその行は残す", func(t *testing.T) {
		t.Parallel()

		c := newLogCapture(10)

		// 切断位置の直前が改行なら先頭行は欠けていないので捨てない.
		_, _ = c.Write([]byte("secret-xyz\nkept line\n"))

		assert.Equal(t, "kept line\n", strings.TrimPrefix(c.String(), logTruncationNotice))
	})

	t.Run("複数回の書き込みを跨いで上限が効く", func(t *testing.T) {
		t.Parallel()

		c := newLogCapture(5)
		for _, s := range []string{"aaa", "bbb", "ccc"} {
			_, _ = c.Write([]byte(s))
		}

		assert.Equal(t, "bbccc", strings.TrimPrefix(c.String(), logTruncationNotice))
	})

	t.Run("バッファの詰め直しを跨いでも末尾が残る", func(t *testing.T) {
		t.Parallel()

		// limit の logCaptureBufferFactor 倍を超えると内部で先頭へ詰め直しが走る.
		// その前後で末尾 limit バイトが保たれることを確認する.
		c := newLogCapture(4)
		for i := range 10 {
			_, _ = c.Write([]byte{byte('0' + i)})
		}

		assert.Equal(t, "6789", strings.TrimPrefix(c.String(), logTruncationNotice))
	})
}

func TestBuilderRedactSecrets(t *testing.T) {
	t.Parallel()

	b := NewBuilder(&config.ResoniteBuildConfig{
		SteamUsername:    "steamuser",
		SteamPassword:    "sup3r-secret",
		HeadlessPassword: "headless-beta-pw",
	}, &config.DockerConfig{})

	got := b.redactSecrets("login steamuser with sup3r-secret and -betapassword headless-beta-pw\n")

	assert.Equal(t, "login *** with *** and -betapassword ***\n", got)
}

func TestSanitizeForStorage(t *testing.T) {
	t.Parallel()

	// Postgres の text は NUL を格納できず、不正な UTF-8 も拒否する.
	// 落とさないと書き込みごと失敗して失敗ログが残らない.
	got := sanitizeForStorage("ok\x00 \xff\xfe tail")

	assert.Equal(t, "ok  tail", got)
	assert.True(t, utf8.ValidString(got))
}

func TestBuilderRedactSecretsSkipsShortValues(t *testing.T) {
	t.Parallel()

	// 短い値まで置換すると無関係な部分文字列を壊してログが読めなくなる.
	b := NewBuilder(&config.ResoniteBuildConfig{
		SteamUsername:    "ab",
		SteamPassword:    "",
		HeadlessPassword: "",
	}, &config.DockerConfig{})

	const line = "abort: build failed\n"

	assert.Equal(t, line, b.redactSecrets(line))
}
