import { useMutation, useQuery } from "@connectrpc/connect-query";
import {
  acceptFriendRequests,
  getFriendRequests,
} from "../../pbgen/hdlctrl/v1/controller-ControllerService_connectquery";
import { Button, Card, CardContent, CardHeader } from "@mui/material";
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
    <Card variant="outlined">
      <CardHeader
        title="フレンドリクエスト"
        action={<RefetchButton refetch={refetch} />}
      />
      <CardContent>
        <ScrollBase height={scrollHeight}>
          <UserList
            data={data?.users ?? []}
            isLoading={isPending}
            renderActions={(user) => (
              <Button
                variant="contained"
                color="primary"
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
