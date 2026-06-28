import { useParams } from "react-router";
import GroupDetailPanel from "../../components/GroupDetailPanel";
import GroupMemberList from "../../components/GroupMemberList";
import RoleList from "../../components/RoleList";
import { useQuery } from "@connectrpc/connect-query";
import { getGroup } from "../../../pbgen/hdlctrl/v1/permission-GroupService_connectquery";
import { GroupType, RoleScope } from "../../../pbgen/hdlctrl/v1/permission_pb";
import { usePermissions } from "../../hooks/usePermissions";
import { PERMISSION_KEYS } from "../../libs/permissionUtils";

export default function GroupDetail() {
  const { id } = useParams();
  const { data } = useQuery(getGroup, { groupId: id ?? "" }, { enabled: !!id });
  const { hasPermission } = usePermissions();

  if (!id) {
    return (
      <div className="container mx-auto p-4">
        <p className="text-destructive">
          NotFound: グループが見つかりませんでした
        </p>
      </div>
    );
  }

  const group = data?.group;
  const canManageRoles =
    !!group &&
    group.type !== GroupType.SYSTEM &&
    hasPermission(id, PERMISSION_KEYS.GROUP_MEMBERS_MANAGE);
  const scope =
    group?.type === GroupType.SYSTEM ? RoleScope.SYSTEM : RoleScope.NORMAL;

  return (
    <div className="container mx-auto p-4 space-y-6">
      <section>
        <h2 className="text-lg font-semibold mb-2">グループ情報</h2>
        <GroupDetailPanel groupId={id} />
      </section>
      <section className="border-t pt-4">
        <h2 className="text-lg font-semibold mb-2">メンバー</h2>
        <GroupMemberList groupId={id} />
      </section>
      <section className="border-t pt-4">
        <h2 className="text-lg font-semibold mb-2">グループ内カスタムロール</h2>
        <RoleList groupId={id} canManage={canManageRoles} scope={scope} />
      </section>
    </div>
  );
}
