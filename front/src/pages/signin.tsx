"use client";
import { SignInPage } from "@toolpad/core/SignInPage";
import { Navigate, useNavigate } from "react-router";
import { useSession, type Session } from "../SessionContext";

export default function SignIn() {
  const { session, setSession } = useSession();
  const navigate = useNavigate();

  if (session) {
    return <Navigate to="/" />;
  }

  return (
    <SignInPage
      providers={[{ id: "credentials", name: "Credentials" }]}
      signIn={async (provider, formData, callbackUrl) => {
        let result;
        try {
          if (provider.id === "credentials") {
            const email = formData?.get("email") as string;
            const password = formData?.get("password") as string;

            if (!email || !password) {
              return { error: "Email and password are required" };
            }

            // TODO: login request
            result = {
              success: true,
              error: null,
              user: {
                displayName: "admin",
                email: "test@baru.dev",
              },
            };
          }

          if (result?.success && result?.user) {
            const userSession: Session = {
              user: {
                name: result.user.displayName || "",
                email: result.user.email || "",
              },
            };
            setSession(userSession);
            navigate(callbackUrl || "/", { replace: true });
            return {};
          }
          return { error: result?.error || "Failed to sign in" };
        } catch (error) {
          return {
            error: error instanceof Error ? error.message : "An error occurred",
          };
        }
      }}
    />
  );
}
