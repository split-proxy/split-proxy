import type { Key } from "react";
import type { TableProps } from "antd";
import {
  DeleteOutlined,
  EditOutlined,
  PlusOutlined,
  SearchOutlined,
} from "@ant-design/icons";
import {
  Button,
  Input,
  Modal,
  Space,
  Table,
  message,
} from "antd";
import { useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import {
  useCallback,
  useEffect,
  useMemo,
  useState,
} from "react";
import { DeleteProxyModal } from "./DeleteProxyModal";
import { getProxies, removeProxy } from "@/shared/api/proxyApi";
import type { ProxyItem } from "@/shared/types";

const DEFAULT_PAGE_SIZE = 100;
const PAGE_SIZE_OPTIONS = [10, 20, 50, 100];
const SEARCH_DEBOUNCE_MS = 400;

export function ProxiesPage() {
  const navigate = useNavigate();
  const { t } = useTranslation();

  const [proxies, setProxies] = useState<ProxyItem[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);

  const [deletingProxy, setDeletingProxy] =
    useState<ProxyItem | null>(null);
  const [deleteLoading, setDeleteLoading] = useState(false);

  const [selectedRowKeys, setSelectedRowKeys] = useState<Key[]>([]);
  const [bulkDeleteLoading, setBulkDeleteLoading] = useState(false);

  const [currentPage, setCurrentPage] = useState(1);
  const [pageSize, setPageSize] = useState(DEFAULT_PAGE_SIZE);

  const [searchInput, setSearchInput] = useState("");
  const [search, setSearch] = useState("");

  // Debounce поиска, чтобы не отправлять запрос при каждом нажатии клавиши.
  useEffect(() => {
    const timeoutId = setTimeout(() => {
      const normalizedSearch = searchInput.trim();

      setSearch((previous) =>
        previous === normalizedSearch ? previous : normalizedSearch,
      );
      setCurrentPage(1);
    }, SEARCH_DEBOUNCE_MS);

    return () => clearTimeout(timeoutId);
  }, [searchInput]);

  const loadProxies = useCallback(async () => {
    setLoading(true);

    try {
      const response = await getProxies({
        page: currentPage,
        pageSize,
        search,
      });

      setProxies(response.results);
      setTotal(response.count);

      return response;
    } catch {
      message.error(t("proxiesPage.proxiesNotLoaded"));
      return undefined;
    } finally {
      setLoading(false);
    }
  }, [currentPage, pageSize, search, t]);

  useEffect(() => {
    void loadProxies();
  }, [loadProxies]);

  const clearSelection = useCallback(() => {
    setSelectedRowKeys([]);
  }, []);

  const handleSearchChange = (value: string) => {
    setSearchInput(value);
    clearSelection();
  };

  const handlePageChange = (page: number, nextPageSize: number) => {
    clearSelection();

    if (nextPageSize !== pageSize) {
      setPageSize(nextPageSize);
      setCurrentPage(1);
      return;
    }

    setCurrentPage(page);
  };

  const remove = async () => {
    if (!deletingProxy) return;

    setDeleteLoading(true);

    try {
      await removeProxy(deletingProxy.id);

      message.success(t("proxiesPage.proxyDeleted"));
      setDeletingProxy(null);
      clearSelection();

      const remainingTotal = Math.max(0, total - 1);
      const maxPage = Math.max(
        1,
        Math.ceil(remainingTotal / pageSize),
      );

      if (currentPage > maxPage) {
        setCurrentPage(maxPage);
      } else {
        await loadProxies();
      }
    } catch (error) {
      message.error(
        error instanceof Error
          ? error.message
          : t("proxiesPage.deletionError"),
      );
    } finally {
      setDeleteLoading(false);
    }
  };

  const bulkRemove = async () => {
    if (selectedRowKeys.length === 0) return;

    setBulkDeleteLoading(true);

    try {
      const selectedIds = selectedRowKeys.map(Number);

      const results = await Promise.allSettled(
        selectedIds.map((id) => removeProxy(id)),
      );

      const successfulIds = selectedIds.filter(
        (_, index) => results[index].status === "fulfilled",
      );

      const failedIds = selectedIds.filter(
        (_, index) => results[index].status === "rejected",
      );

      const failedCount = failedIds.length;

      setSelectedRowKeys(failedIds);

      if (failedCount === 0) {
        message.success(
          t("proxiesPage.bulkDeleted", {
            count: successfulIds.length,
          }),
        );
      } else if (successfulIds.length > 0) {
        message.warning(
          t("proxiesPage.bulkDeletePartial", {
            success: successfulIds.length,
            failed: failedCount,
          }),
        );
      } else {
        message.error(t("proxiesPage.bulkDeleteFailed"));
      }

      // После удаления обновляем страницу с учётом поиска и пагинации.
      const remainingTotal = Math.max(
        0,
        total - successfulIds.length,
      );

      const maxPage = Math.max(
        1,
        Math.ceil(remainingTotal / pageSize),
      );

      if (currentPage > maxPage) {
        setCurrentPage(maxPage);
      } else {
        await loadProxies();
      }
    } finally {
      setBulkDeleteLoading(false);
    }
  };

  const confirmBulkRemove = () => {
    if (selectedRowKeys.length === 0) return;

    Modal.confirm({
      title: t("proxiesPage.bulkDeleteTitle"),
      content: t("proxiesPage.bulkDeleteConfirm", {
        count: selectedRowKeys.length,
      }),
      okText: t("common.delete"),
      cancelText: t("common.cancel"),
      okButtonProps: { danger: true },
      onOk: bulkRemove,
    });
  };

  const columns: TableProps<ProxyItem>["columns"] = useMemo(
    () => [
      {
        title: t("proxiesPage.login"),
        dataIndex: "username",
        key: "username",
      },
      {
        title: t("common.updatedAt"),
        dataIndex: "updated_at",
        key: "updated_at",
        responsive: ["md"],
        render: (value: number) =>
          new Date(value * 1000).toLocaleString(),
      },
      {
        title: t("common.createdAt"),
        dataIndex: "created_at",
        key: "created_at",
        responsive: ["md"],
        render: (value: number) =>
          new Date(value * 1000).toLocaleString(),
      },
      {
        title: t("common.actions"),
        key: "actions",
        width: 120,
        render: (_, proxy) => (
          <Space>
            <Button
              aria-label={t("common.edit")}
              icon={<EditOutlined />}
              onClick={() => navigate(`/proxies/${proxy.id}/edit`)}
            />

            <Button
              aria-label={t("common.delete")}
              danger
              icon={<DeleteOutlined />}
              onClick={() => setDeletingProxy(proxy)}
            />
          </Space>
        ),
      },
    ],
    [navigate, t],
  );

  return (
    <div className="proxies-page">
      <div className="page-heading">
        <div>
          <h1>{t("proxiesPage.h1")}</h1>
        </div>

        <Button
          type="primary"
          icon={<PlusOutlined />}
          onClick={() => navigate("/proxies/new")}
        >
          {t("proxiesPage.addProxy")}
        </Button>
      </div>

      <div className="proxies-toolbar">
        <Input
          allowClear
          value={searchInput}
          prefix={<SearchOutlined />}
          placeholder={t("proxiesPage.searchPlaceholder")}
          onChange={(event) => handleSearchChange(event.target.value)}
          style={{ maxWidth: 360 }}
        />
      </div>

      <Table<ProxyItem>
        rowKey="id"
        columns={columns}
        dataSource={proxies}
        loading={loading || bulkDeleteLoading}
        scroll={{ x: 480 }}
        rowSelection={{
          selectedRowKeys,
          onChange: (keys) => setSelectedRowKeys(keys),
          getCheckboxProps: () => ({
            disabled: bulkDeleteLoading,
          }),
        }}
        pagination={{
          current: currentPage,
          pageSize,
          total,
          showSizeChanger: true,
          pageSizeOptions: PAGE_SIZE_OPTIONS.map(String),
          showTotal: (count, range) =>
            t("common.paginationTotal", {
              from: range[0],
              to: range[1],
              total: count,
            }),
          onChange: handlePageChange,
        }}
      />

      {selectedRowKeys.length > 0 && (
        <div className="proxy-bulk-footer">
          <span>
            {t("proxiesPage.selectedProxies", {
              count: selectedRowKeys.length,
            })}
          </span>

          <Space wrap>
            <Button
              disabled={bulkDeleteLoading}
              onClick={clearSelection}
            >
              {t("proxiesPage.clearSelection")}
            </Button>

            <Button
              danger
              icon={<DeleteOutlined />}
              loading={bulkDeleteLoading}
              onClick={confirmBulkRemove}
            >
              {t("proxiesPage.bulkDelete")}
            </Button>
          </Space>
        </div>
      )}

      <DeleteProxyModal
        open={deletingProxy !== null}
        proxy={deletingProxy}
        loading={deleteLoading}
        onCancel={() => setDeletingProxy(null)}
        onConfirm={remove}
      />
    </div>
  );
}
