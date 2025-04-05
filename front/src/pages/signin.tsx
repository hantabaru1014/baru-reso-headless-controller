"use client";
import { SignInPage } from "@toolpad/core/SignInPage";
import { Navigate, useNavigate } from "react-router";
import { useAtom } from "jotai";
import { sessionAtom } from "../atoms/sessionAtom";
import { useAuth } from "../hooks/useAuth";

export default function SignIn() {
  const [session] = useAtom(sessionAtom);
  const navigate = useNavigate();
  const { signIn } = useAuth("/");
  const query = new URLSearchParams(location.search);
  const queryCallbackUrl = query.get("callbackUrl");

  if (session) {
    return <Navigate to={queryCallbackUrl || "/"} />;
  }

  return (
    <SignInPage
      providers={[{ id: "credentials", name: "Credentials" }]}
      signIn={async (provider, formData) => {
        if (provider.id === "credentials") {
          const email = formData?.get("email") as string;
          const password = formData?.get("password") as string;

          const response = await signIn(email, password);
          if (response.ok) {
            navigate(queryCallbackUrl || "/", { replace: true });
            return { error: undefined };
          } else {
            return { error: response.error };
          }
        }
        return { error: "Unknown provider" };
      }}
    />
  );
}
