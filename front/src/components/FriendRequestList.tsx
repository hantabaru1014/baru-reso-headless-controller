import { useMutation, useQuery } from "@connectrpc/connect-query";
import {
  acceptFriendRequests,
  getFriendRequests,
} from "../../pbgen/hdlctrl/v1/controller-ControllerService_connectquery";
import { Button } from "./ui/button";
import { Card, CardContent, CardHeader } from "./ui/card";
import RefetchButton from "./base/RefetchButton";
import UserList from "./base/UserList";
import ScrollBase from "./base/ScrollBase";

export default function FriendRequestList({
  hostId,
  scrollHeight = "15rem",
}: {
  hostId: string;
  scrollHeight?: string;
}) {
  const { data, isPending, refetch } = useQuery(getFriendRequests, { hostId });
  const { mutateAsync: mutateAcceptFriendRequest } =
    useMutation(acceptFriendRequests);

  return (
    <Card>
      <CardHeader>
        <div className="flex items-center justify-between">
          <h3 className="text-lg font-semibold">フレンドリクエスト</h3>
          <RefetchButton refetch={refetch} />
        </div>
      </CardHeader>
      <CardContent>
        <ScrollBase height={scrollHeight}>
          <UserList
            data={data?.users ?? []}
            isLoading={isPending}
            renderActions={(user) => (
              <Button
                onClick={() => {
                  mutateAcceptFriendRequest({ hostId, userIds: [user.id] });
                }}
              >
                承認
              </Button>
            )}
          />
        </ScrollBase>
      </CardContent>
    </Card>
  );
}
