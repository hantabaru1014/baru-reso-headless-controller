import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from "./base/dialog";
import { Button } from "./base/button";
import { Input } from "./base/input";
import { Search, Check } from "lucide-react";
import { useMutation, useQuery } from "@connectrpc/connect-query";
import {
  banUser,
  getSessionDetails,
  inviteUser,
  kickUser,
  listUsersInSession,
  searchUserInfo,
  updateUserRole,
} from "../../pbgen/hdlctrl/v1/controller-ControllerService_connectquery";
import { UserRoles } from "../constants";
import EditableSelectField from "./base/EditableSelectField";
import { useState } from "react";
import UserList from "./base/UserList";
import RefetchButton from "./base/RefetchButton";
import ScrollBase from "./base/ScrollBase";
import { ColumnDef } from "@tanstack/react-table";
import { UserInSession } from "front/pbgen/headless/v1/headless_pb";
import { DataTable } from "./base";
import { toast } from "sonner";

function UserInviteDialog({
  isOpen: open,
  onClose,
  hostId,
  sessionId,
}: {
  isOpen: boolean;
  onClose: () => void;
  hostId?: string;
  sessionId?: string;
}) {
  const [query, setQuery] = useState("");
  const {
    data: searchResult,
    mutateAsync: mutateSearch,
    isPending: isPendingSearch,
  } = useMutation(searchUserInfo);
  const { mutateAsync: mutateInviteUser } = useMutation(inviteUser);

  const handleQueryChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const value = e.target.value.toLowerCase();
    setQuery(value);
    const isId = value.startsWith("u-");
    mutateSearch({
      hostId,
      parameters: {
        user: {
          case: isId ? "userId" : "userName",
          value,
        },
        onlyInContacts: true,
        partialMatch: true,
      },
    });
  };

  const handleInviteUser = async (userId: string) => {
    try {
      await mutateInviteUser({
        hostId,
        sessionId: sessionId,
        user: {
          case: "userId",
          value: userId,
        },
      });
      toast.success("ユーザーを招待しました");
    } catch (e) {
      toast.error(`ユーザーの招待に失敗しました: ${e}`);
    }
  };

  return (
    <Dialog open={open} onOpenChange={() => onClose()}>
      <DialogContent className="max-w-md">
        <DialogHeader>
          <DialogTitle>ユーザーを招待</DialogTitle>
        </DialogHeader>
        <div className="space-y-4">
          <div className="relative">
            <Search className="absolute left-3 top-3 h-4 w-4 text-gray-400" />
            <Input
              placeholder="ユーザーID/名"
              value={query}
              onChange={handleQueryChange}
              className="pl-10"
            />
          </div>
          <ScrollBase height="60vh">
            <UserList
              data={searchResult?.users || []}
              isLoading={isPendingSearch}
              renderActions={(user) => (
                <Button onClick={() => handleInviteUser(user.id)}>招待</Button>
              )}
            />
          </ScrollBase>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={() => onClose()}>
            閉じる
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

export default function SessionUserList({ sessionId }: { sessionId: string }) {
  const { data: sessionDetail } = useQuery(getSessionDetails, {
    sessionId,
  });
  const hostId = sessionDetail?.session?.hostId;
  const { data, isPending, refetch } = useQuery(listUsersInSession, {
    hostId,
    sessionId,
  });
  const { mutateAsync: mutateUpdateRole } = useMutation(updateUserRole);
  const { mutateAsync: mutateKickUser } = useMutation(kickUser);
  const { mutateAsync: mutateBanUser } = useMutation(banUser);
  const [isOpenInviteDialog, setIsOpenInviteDialog] = useState(false);

  const handleUpdateRole = async (userId: string, role: string) => {
    try {
      await mutateUpdateRole({
        hostId,
        parameters: {
          sessionId,
          user: {
            case: "userId",
            value: userId,
          },
          role,
        },
      });
      setTimeout(() => {
        refetch();
      }, 500);
      return { ok: true };
    } catch (e) {
      return { ok: false, error: e instanceof Error ? e.message : `${e}` };
    }
  };

  const handleKickUser = async (userId: string) => {
    try {
      await mutateKickUser({
        hostId,
        parameters: {
          sessionId,
          user: {
            case: "userId",
            value: userId,
          },
        },
      });
      setTimeout(() => {
        refetch();
      }, 500);
      return { ok: true };
    } catch (e) {
      return { ok: false, error: e instanceof Error ? e.message : `${e}` };
    }
  };

  const handleBanUser = async (userId: string) => {
    try {
      await mutateBanUser({
        hostId,
        parameters: {
          sessionId,
          user: {
            case: "userId",
            value: userId,
          },
        },
      });
      setTimeout(() => {
        refetch();
      }, 500);
      return { ok: true };
    } catch (e) {
      return { ok: false, error: e instanceof Error ? e.message : `${e}` };
    }
  };

  const columns: ColumnDef<UserInSession>[] = [
    {
      accessorKey: "name",
      header: "ユーザー名",
    },
    {
      accessorKey: "role",
      header: "権限",
      cell: ({ row }) => (
        <EditableSelectField<string>
          selectedId={row.original.role}
          options={UserRoles.map((r) => r)}
          onSave={(v) => handleUpdateRole(row.original.id, v)}
        />
      ),
    },
    {
      accessorKey: "isPresent",
      header: "離席中",
      cell: ({ cell }) =>
        !cell.getValue<boolean>() ? <Check className="h-4 w-4" /> : null,
    },
    {
      id: "actions",
      header: "操作",
      cell: ({ row }) => (
        <div className="space-x-2">
          <Button
            variant="outline"
            size="sm"
            onClick={() => handleKickUser(row.original.id)}
          >
            Kick
          </Button>
          <Button
            variant="destructive"
            size="sm"
            onClick={() => handleBanUser(row.original.id)}
          >
            Ban
          </Button>
        </div>
      ),
    },
  ];

  return (
    <>
      <div className="space-y-4">
        <div className="flex justify-end space-x-2">
          <Button onClick={() => setIsOpenInviteDialog(true)}>
            ユーザー招待
          </Button>
          <RefetchButton refetch={refetch} />
        </div>
        <DataTable
          columns={columns}
          data={data?.users || []}
          isLoading={isPending}
        />
      </div>
      <UserInviteDialog
        isOpen={isOpenInviteDialog}
        onClose={() => {
          setIsOpenInviteDialog(false);
          refetch();
        }}
        hostId={hostId}
        sessionId={sessionId}
      />
    </>
  );
}
