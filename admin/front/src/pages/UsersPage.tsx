import { DeleteOutlined, EditOutlined, PlusOutlined, CheckCircleOutlined, CloseCircleOutlined } from "@ant-design/icons";
import { Button, Space, Table, Tag, message } from "antd";
import type { TableProps } from "antd";
import { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { getUsers, removeUser } from "@/shared/api/userApi";
import { DeleteUserModal } from "../components/DeleteUserModal";
import type { User } from "@/shared/types";
import { useTranslation } from "react-i18next";

export function UsersPage() {
  const navigate = useNavigate();
  const { t, i18n } = useTranslation();
  const [users, setUsers] = useState<User[]>([]);
  const [loading, setLoading] = useState(true);
  const [deletingUser, setDeletingUser] = useState<User | null>(null);
  const [deleteLoading, setDeleteLoading] = useState(false);

  const loadUsers = async () => {
    setLoading(true);

    try {
      const response = await getUsers();
      setUsers(response.results);
    } catch {
      message.error(t("usersPage.usersNotLoaded"));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void loadUsers();
  }, []);

  const remove = async () => {
    if (!deletingUser) return;

    setDeleteLoading(true);
    try {
      if (deletingUser.is_superuser) {
        message.warning(t("usersPage.cantDeleteSuperUser"));
      } else {
        await removeUser(deletingUser.id);
        message.success(t("usersPage.userDeleted"));
        setDeletingUser(null);
        await loadUsers();
      }
    } catch (e) {
      message.error(e instanceof Error ? e.message : t("usersPage.deletionError"));
    } finally {
      setDeleteLoading(false);
    }
  };

  const columns: TableProps<User>["columns"] = [
    {
      title: t("common.login"),
      dataIndex: "username",
      key: "username",
    },
    {
      title: t("common.createdAt"),
      dataIndex: "created_at",
      key: "created_at",
      responsive: ["md"],
      render: (value: number) =>
        new Date(value * 1000).toLocaleString("ru-RU"),
    },
    {
      title: t("usersPage.isSuperuser"),
      dataIndex: "is_superuser",
      key: "is_superuser",
      responsive: ["md"],
      align: "center",
      render: (value: boolean) => (
        value ? (
          <CheckCircleOutlined style={{ color: "#52C41A" }}/>
        ) : (
          <CloseCircleOutlined style={{ color: "#ff4d4f" }}/>
        )
      )
    },
    {
      title: t("common.actions"),
      key: "actions",
      width: 120,
      render: (_, user) => (
        <Space>
          <Button
            aria-label={t("common.edit")}
            icon={<EditOutlined />}
            onClick={() => navigate(`/users/${user.id}/edit`)}
          />

          {!user.is_superuser && (
            <Button
              aria-label={t("common.delete")}
              danger
              icon={<DeleteOutlined />}
              onClick={() => setDeletingUser(user)}
            />
          )}
        </Space>
      ),
    },
  ];

  return (
    <div className="users-page">
      <div className="page-heading">
        <div>
          <h1>{t("usersPage.admins")}</h1>
        </div>

        <Button
          type="primary"
          icon={<PlusOutlined />}
          onClick={() => navigate("/users/new")}
        >
          {t("usersPage.addUser")}
        </Button>
      </div>

      <Table<User>
        rowKey="id"
        columns={columns}
        dataSource={users}
        loading={loading}
        scroll={{ x: 480 }}
        pagination={{ pageSize: 10, hideOnSinglePage: true }}
      />

      <DeleteUserModal
        open={deletingUser !== null}
        user={deletingUser}
        loading={deleteLoading}
        onCancel={() => setDeletingUser(null)}
        onConfirm={remove}
      />
    </div>
  );
}
