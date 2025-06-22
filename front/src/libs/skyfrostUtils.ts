// もしURLがresdb形式であれば、解決されたURLを返す
export function resolveResdbUrl(url: string | undefined) {
  if (!url) return undefined;
  const match = url.match(/resdb:\/\/\/([^.]+)/);
  if (!match || match.length < 2) return undefined;

  return `https://assets.resonite.com/${match[1]}`;
}

// URLがresdbでもhttpでも解決してhttp形式のURLを返す
export function resolveUrl(url: string | undefined) {
  const resolved = resolveResdbUrl(url);
  return resolved ? resolved : url;
}
