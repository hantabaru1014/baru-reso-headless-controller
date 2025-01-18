import { useAtom } from "jotai";
import { sessionAtom, sessionRefreshTokenAtom } from "../atoms/sessionAtom";
import { createConnectTransport } from "@connectrpc/connect-web";
import { callUnaryMethod } from "@connectrpc/connect-query";
import {
  getTokenByPassword,
  refreshToken as refreshTokenRpc,
} from "../../pbgen/hdlctrl/v1/user-UserService_connectquery";
import { useCallback, useEffect, useMemo } from "react";
import { jwtDecode, JwtPayload as DefaultJwtPayload } from "jwt-decode";

type JwtPayload = DefaultJwtPayload & {
  user_id: string;
};

const decodeJwt = (token: string) => jwtDecode<JwtPayload>(token);

export const useAuth = (baseUrl: string) => {
  const [session, setSession] = useAtom(sessionAtom);
  const [refreshToken, setRefreshToken] = useAtom(sessionRefreshTokenAtom);

  const transportWithRefreshToken = useMemo(
    () =>
      createConnectTransport({
        baseUrl,
        fetch: async (input: RequestInfo | URL, init?: RequestInit) => {
          const headers = new Headers(init?.headers);
          if (refreshToken) {
            headers.set("authorization", `Bearer ${refreshToken}`);
          }
          return fetch(input, {
            ...init,
            headers,
          });
        },
      }),
    [baseUrl, refreshToken],
  );

  useEffect(() => {
    (async () => {
      if (!session && refreshToken) {
        const refreshResponse = await callUnaryMethod(
          transportWithRefreshToken,
          refreshTokenRpc,
          {},
        );
        if (refreshResponse.token) {
          const payload = decodeJwt(refreshResponse.token);
          setSession({
            token: refreshResponse.token,
            user: {
              name: payload.user_id,
              email: payload.user_id,
            },
          });
          setRefreshToken(refreshResponse.refreshToken);
        }
      }
    })();
  }, []);

  const configuredFetch = useCallback(
    async (input: RequestInfo | URL, init?: RequestInit) => {
      const headers = new Headers(init?.headers);
      if (session) {
        headers.set("authorization", `Bearer ${session.token}`);
      }

      const response = await fetch(input, {
        ...init,
        headers,
      });

      if (response.status === 401 && refreshToken) {
        const refreshResponse = await callUnaryMethod(
          transportWithRefreshToken,
          refreshTokenRpc,
          {},
        );

        if (refreshResponse.token) {
          const token = refreshResponse.token;

          setSession((prev) => prev && { ...prev, token });
          setRefreshToken(refreshResponse.refreshToken);

          headers.set("authorization", `Bearer ${token}`);
          return await fetch(input, {
            ...init,
            headers,
          });
        } else {
          setSession(null);
          setRefreshToken(null);
        }
      }

      return response;
    },
    [
      session,
      refreshToken,
      setSession,
      setRefreshToken,
      transportWithRefreshToken,
    ],
  );

  const signIn = useCallback(
    async (id: string, password: string) => {
      try {
        const response = await callUnaryMethod(
          transportWithRefreshToken,
          getTokenByPassword,
          {
            id,
            password,
          },
        );

        if (response.token) {
          const payload = decodeJwt(response.token);
          setSession({
            token: response.token,
            user: {
              name: payload.user_id,
              email: payload.user_id,
            },
          });
          setRefreshToken(response.refreshToken);

          return { ok: true };
        } else {
          return { ok: false, error: "Invalid credentials" };
        }
      } catch (error) {
        if (error instanceof Error) {
          return { ok: false, error: error.message };
        } else {
          return { ok: false, error: "An unknown error occurred" };
        }
      }
    },
    [setSession, setRefreshToken, transportWithRefreshToken],
  );

  const signOut = useCallback(() => {
    setSession(null);
    setRefreshToken(null);
  }, [setSession, setRefreshToken]);

  return { configuredFetch, signIn, signOut };
};
