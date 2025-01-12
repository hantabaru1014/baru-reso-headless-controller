import * as React from "react";

export interface Session {
  user: {
    name?: string;
    email?: string;
    image?: string;
  };
}

interface SessionContextType {
  session: Session | null;
  setSession: (session: Session | null) => void;
}

const SessionContext = React.createContext<SessionContextType>({
  session: null,
  setSession: () => {},
});

export default SessionContext;

export const useSession = () => React.useContext(SessionContext);
