import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from "./base/dialog";
import { Button } from "./base/button";
import { Card, CardContent, CardHeader } from "./base/card";
import { Input } from "./base/input";
import { Label } from "./base/label";
import UserList from "./base/UserList";
import { useMutation, useQuery } from "@connectrpc/connect-query";
import {
  createHeadlessAccount,
  listHeadlessAccounts,
} from "../../pbgen/hdlctrl/v1/controller-ControllerService_connectquery";
import RefetchButton from "./base/RefetchButton";
import { useEffect, useState } from "react";
import { toast } from "sonner";

function NewAccountDialog({
  open,
  onClose,
}: {
  open: boolean;
  onClose: () => void;
}) {
  const { mutateAsync: mutateCreateAccount, isPending } = useMutation(
    createHeadlessAccount,
  );
  const [userId, setUserId] = useState("U-");
  const [credential, setCredential] = useState("");
  const [password, setPassword] = useState("");

  useEffect(() => {
    if (open) {
      setUserId("U-");
      setCredential("");
      setPassword("");
    }
  }, [open]);

  return (
    <Dialog open={open} onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="sm:max-w-[425px]">
        <DialogHeader>
          <DialogTitle>ヘッドレスアカウントを追加</DialogTitle>
        </DialogHeader>
        <div className="grid gap-4 py-4">
          <div className="grid gap-2">
            <Label htmlFor="userId">User ID</Label>
            <Input
              id="userId"
              value={userId}
              onChange={(e) => setUserId(e.target.value)}
            />
          </div>
          <div className="grid gap-2">
            <Label htmlFor="credential">Credential</Label>
            <Input
              id="credential"
              value={credential}
              onChange={(e) => setCredential(e.target.value)}
            />
          </div>
          <div className="grid gap-2">
            <Label htmlFor="password">Password</Label>
            <Input
              id="password"
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
            />
          </div>
        </div>
        <DialogFooter>
          <Button
            onClick={async () => {
              try {
                await mutateCreateAccount({
                  resoniteUserId: userId,
                  credential,
                  password,
                });
                toast.success("アカウントを追加しました");
              } catch (e) {
                toast.error(
                  e instanceof Error
                    ? e.message
                    : "アカウントの追加に失敗しました",
                );
                return;
              }
              onClose();
            }}
            disabled={isPending}
          >
            追加
          </Button>
          <Button variant="outline" onClick={() => onClose()}>
            キャンセル
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

export default function HeadlessAccountList() {
  const { data, isPending, refetch } = useQuery(listHeadlessAccounts);
  const [isDialogOpen, setIsDialogOpen] = useState(false);

  const handleNewAccount = () => {
    setIsDialogOpen(true);
  };

  const handleCloseDialog = () => {
    setIsDialogOpen(false);
    refetch();
  };

  return (
    <>
      <Card>
        <CardHeader>
          <div className="flex items-center justify-between">
            <h3 className="text-lg font-semibold">ヘッドレスアカウント</h3>
            <div className="flex gap-2">
              <RefetchButton refetch={refetch} />
              <Button onClick={handleNewAccount}>追加</Button>
            </div>
          </div>
        </CardHeader>
        <CardContent>
          <UserList
            data={
              data?.accounts.map((account) => ({
                id: account.userId,
                name: account.userName,
                iconUrl: account.iconUrl,
              })) ?? []
            }
            isLoading={isPending}
          />
        </CardContent>
      </Card>
      <NewAccountDialog open={isDialogOpen} onClose={handleCloseDialog} />
    </>
  );
}
