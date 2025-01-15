"use client";
import { SignInPage } from "@toolpad/core/SignInPage";
import { Navigate, useNavigate } from "react-router";
import { useAtom } from "jotai";
import { sessionAtom } from "../atoms/sessionAtom";
import { useAuth } from "../hooks/useAuthConfiguredFetch";

export default function SignIn() {
  const [session] = useAtom(sessionAtom);
  const navigate = useNavigate();
  const { signIn } = useAuth("/");

  if (session) {
    return <Navigate to="/" />;
  }

  return (
    <SignInPage
      providers={[{ id: "credentials", name: "Credentials" }]}
      signIn={async (provider, formData, callbackUrl) => {
        if (provider.id === "credentials") {
          const email = formData?.get("email") as string;
          const password = formData?.get("password") as string;

          const response = await signIn(email, password);
          if (response.ok) {
            navigate(callbackUrl || "/", { replace: true });
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
