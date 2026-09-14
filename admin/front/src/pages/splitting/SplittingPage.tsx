import type { TableProps } from "antd";
import { useConfig } from "@/shared/config/ConfigContext";

import {
  DeleteOutlined,
  PlusOutlined,
  SearchOutlined,
} from "@ant-design/icons";

import {
  Button,
  Card,
  Checkbox,
  Input,
  Select,
  Space,
  Table,
  Tabs,
  Tag,
  Typography,
  message,
} from "antd";

import {
  useCallback,
  useEffect,
  useMemo,
  useState,
} from "react";

import { useSearchParams } from "react-router-dom";

import { useTranslation } from "react-i18next";

import { getGroups } from "@/shared/api/groupApi";

import {
  addCidrs,
  addDomains,
  deleteCidr,
  deleteDomain,
  getDefaultRoute,
  getListCidr,
  getListDomains,
  updateDefaultRoute,
  updateCidr,
  updateDomain,
} from "@/shared/api/routingApi";

import type { GroupItem } from "@/shared/types";

type DomainRule = {
  pattern: string;
  group: number | null;
};

type CidrRule = {
  cidr: string;
  worker_group: number | null;
};

type DefaultRoute = {
  worker_group: number;
};

const SEARCH_DEBOUNCE_MS = 400;


const isValidDomain = (value: string) => {
  const domain = value.trim();

  if (!domain) {
    return false;
  }

  const isWildcard = domain.startsWith("*.");

  // Wildcard разрешён только в формате *.example.org
  if (domain.includes("*") && !isWildcard) {
    return false;
  }

  const domainWithoutWildcard = isWildcard
    ? domain.slice(2)
    : domain;

  const labels = domainWithoutWildcard.split(".");

  if (labels.length < 2) {
    return false;
  }

  return labels.every(
    (label) =>
      label.length >= 1 &&
      label.length <= 63 &&
      !label.startsWith("-") &&
      !label.endsWith("-") &&
      /^[\p{L}\p{N}-]+$/u.test(label),
  );
};

const isValidCidr = (value: string) => {
  const cidr = value.trim();

  const match = cidr.match(
    /^(\d{1,3}\.){3}\d{1,3}\/(\d|[12]\d|3[0-2])$/,
  );

  if (!match) {
    return false;
  }

  const [ip] = cidr.split("/");

  return ip
    .split(".")
    .every(
      (part) =>
        Number(part) >= 0 &&
        Number(part) <= 255,
    );
};

export function SplittingPage() {
  const { t } = useTranslation();
  const { config, loading } = useConfig();
  const [searchParams, setSearchParams] = useSearchParams();
  const activeTab = searchParams.get("tab") ?? "default";

  /*
   * --------------------------------------------------------------------------
   * Data
   * --------------------------------------------------------------------------
   */

  const [domains, setDomains] = useState<DomainRule[]>([]);
  const [cidrs, setCidrs] = useState<CidrRule[]>([]);
  const [groups, setGroups] = useState<GroupItem[]>([]);
  const [defaultRoute, setDefaultRoute] = useState<number | null>(null);

  /*
   * --------------------------------------------------------------------------
   * Pagination
   * --------------------------------------------------------------------------
   */

  const [domainPage, setDomainPage] = useState(1);
  const [domainPageSize, setDomainPageSize] = useState(10);
  const [domainTotal, setDomainTotal] = useState(0);

  const [cidrPage, setCidrPage] = useState(1);
  const [cidrPageSize, setCidrPageSize] = useState(10);
  const [cidrTotal, setCidrTotal] = useState(0);

  /*
   * --------------------------------------------------------------------------
   * Search
   * --------------------------------------------------------------------------
   */

  const [domainSearchInput, setDomainSearchInput] =
    useState("");

  const [domainSearch, setDomainSearch] =
    useState("");

  const [cidrSearchInput, setCidrSearchInput] =
    useState("");

  const [cidrSearch, setCidrSearch] =
    useState("");

  /*
   * --------------------------------------------------------------------------
   * Loading
   * --------------------------------------------------------------------------
   */

  const [loadingDomains, setLoadingDomains] =
    useState(true);

  const [loadingCidrs, setLoadingCidrs] =
    useState(true);

  const [updatingDefaultRoute, setLoadingDefaultRoute] =
    useState(false);

  const [loadingGroups, setLoadingGroups] =
    useState(true);

  /*
   * --------------------------------------------------------------------------
   * Add forms
   * --------------------------------------------------------------------------
   */

  const [newDomains, setNewDomains] =
    useState("");

  const [newCidr, setNewCidr] =
    useState("");

  const [domainGroup, setDomainGroup] =
    useState<number | null>(null);

  const [cidrGroup, setCidrGroup] =
    useState<number | null>(null);

  const [includeSubdomains, setIncludeSubdomains] =
    useState(false);

  const [addingDomain, setAddingDomain] =
    useState(false);

  const [addingCidr, setAddingCidr] =
    useState(false);

  /*
   * --------------------------------------------------------------------------
   * Delete
   * --------------------------------------------------------------------------
   */

  const [deletingDomain, setDeletingDomain] =
    useState<string | null>(null);

  const [deletingCidr, setDeletingCidr] =
    useState<string | null>(null);

  /*
   * --------------------------------------------------------------------------
   * Groups
   * --------------------------------------------------------------------------
   */

  const groupOptions = useMemo(
    () =>
      groups.map((group) => ({
        value: group.id,
        label: group.group_name,
      })),
    [groups],
  );
  const defaultRouteOptions = useMemo(
    () => [
      {
        value: 0,
        label: config?.default_group_name
      },
      ...groupOptions,
    ],
    [groupOptions],
  );

  /*
   * --------------------------------------------------------------------------
   * Load groups
   * --------------------------------------------------------------------------
   */

  const loadGroups = useCallback(async () => {
    setLoadingGroups(true);

    try {
      const response = await getGroups();

      setGroups(response.results);
    } catch (e) {
      message.error(
        e instanceof Error
          ? e.message
          : t("routingPage.groupsNotLoaded"),
      );
    } finally {
      setLoadingGroups(false);
    }
  }, [t]);

  /*
   * --------------------------------------------------------------------------
   * Load domains
   * --------------------------------------------------------------------------
   */

  const loadDomains = useCallback(async () => {
    setLoadingDomains(true);

    try {
      const response = await getListDomains({
        page: domainPage,
        pageSize: domainPageSize,
        search: domainSearch,
      });

      setDomains(response.results);
      setDomainTotal(response.count);
    } catch (e) {
      message.error(
        e instanceof Error
          ? e.message
          : t("routingPage.domainsNotLoaded"),
      );
    } finally {
      setLoadingDomains(false);
    }
  }, [
    domainPage,
    domainPageSize,
    domainSearch,
    t,
  ]);

  /*
   * --------------------------------------------------------------------------
   * Load CIDRs
   * --------------------------------------------------------------------------
   */

  const loadCidrs = useCallback(async () => {
    setLoadingCidrs(true);

    try {
      const response = await getListCidr({
        page: cidrPage,
        pageSize: cidrPageSize,
        search: cidrSearch,
      });

      setCidrs(response.results);
      setCidrTotal(response.count);
    } catch (e) {
      message.error(
        e instanceof Error
          ? e.message
          : t("routingPage.cidrsNotLoaded"),
      );
    } finally {
      setLoadingCidrs(false);
    }
  }, [
    cidrPage,
    cidrPageSize,
    cidrSearch,
    t,
  ]);
  /*
   * --------------------------------------------------------------------------
   * Load DefaultRouter
   * --------------------------------------------------------------------------
   */

  const loadDefaultRoute = useCallback(async () => {
    setLoadingDefaultRoute(true);

    try {
      const response = await getDefaultRoute();

      setDefaultRoute(response.worker_group);
    } catch (e) {
      message.error(
        e instanceof Error
          ? e.message
          : t("routingPage.defaultRouteNotLoaded"),
      );
    } finally {
      setLoadingDefaultRoute(false);
    }
  }, [t]);

  /*
   * --------------------------------------------------------------------------
   * Initial / reactive loading
   * --------------------------------------------------------------------------
   */

  useEffect(() => {
    void loadGroups();
  }, [loadGroups]);

  useEffect(() => {
    void loadDomains();
  }, [loadDomains]);

  useEffect(() => {
    void loadCidrs();
  }, [loadCidrs]);

  useEffect(() => {
    void loadDefaultRoute();
  }, [loadDefaultRoute]);

  /*
   * --------------------------------------------------------------------------
   * Search debounce
   * --------------------------------------------------------------------------
   */

  useEffect(() => {
    const timeout = window.setTimeout(() => {
      setDomainSearch(domainSearchInput.trim());
      setDomainPage(1);
    }, SEARCH_DEBOUNCE_MS);

    return () => {
      window.clearTimeout(timeout);
    };
  }, [domainSearchInput]);

  useEffect(() => {
    const timeout = window.setTimeout(() => {
      setCidrSearch(cidrSearchInput.trim());
      setCidrPage(1);
    }, SEARCH_DEBOUNCE_MS);

    return () => {
      window.clearTimeout(timeout);
    };
  }, [cidrSearchInput]);

  /*
   * --------------------------------------------------------------------------
   * Add domain
   * --------------------------------------------------------------------------
   */

  const handleAddDomains = async () => {
    const patterns = newDomains
      .split(/\r?\n/)
      .map((cidr) => cidr.trim())
      .filter(Boolean);

    if (patterns.length === 0) {
      message.error(t("routingPage.invalidDomains"));
      return;
    }

    const invalidPatterns = patterns.find((pattern) => !isValidDomain(pattern));

    if (invalidPatterns) {
      message.error(
        t("routingPage.invalidDomains"),
      );
      return;
    }

    if (domainGroup === null) {
      message.error(
        t("routingPage.selectGroup"),
      );
      return;
    }

    const patternsWithSubdomains = includeSubdomains ? patterns.flatMap((pattern) => [pattern, `*.${pattern}`]) : patterns;

    setAddingDomain(true);

    try {
      await addDomains(
        patternsWithSubdomains,
        domainGroup,
      );

      message.success(
        t("routingPage.domainAdded"),
      );

      setNewDomains("");
      setDomainGroup(null);
      setIncludeSubdomains(false);

      setDomainPage(1);

      if (domainPage === 1) {
        await loadDomains();
      }
    } catch (e) {
      message.error(
        e instanceof Error
          ? e.message
          : t("routingPage.domainAddError"),
      );
    } finally {
      setAddingDomain(false);
    }
  };

  /*
   * --------------------------------------------------------------------------
   * Add CIDR
   * --------------------------------------------------------------------------
   */

  const handleAddCidrs = async () => {
    const cidrList = newCidr
      .split(/\r?\n/)
      .map((cidr) => cidr.trim())
      .filter(Boolean);

    if (cidrList.length === 0) {
      message.error(t("routingPage.invalidCidr"));
      return;
    }

    const hasInvalidCidr = cidrList.some(
      (cidr) => !isValidCidr(cidr),
    );

    if (hasInvalidCidr) {
      message.error(t("routingPage.invalidCidr"));
      return;
    }

    if (cidrGroup === null) {
      message.error(t("routingPage.selectGroup"));
      return;
    }

    setAddingCidr(true);

    try {
      // Здесь вызывается именно API addCidrs
      await addCidrs(cidrList, cidrGroup);

      message.success(t("routingPage.cidrAdded"));

      setNewCidr("");
      setCidrGroup(null);

      // Если мы уже на первой странице —
      // просто обновляем данные.
      if (cidrPage === 1) {
        await loadCidrs();
      } else {
        // При переходе на первую страницу useEffect
        // сам вызовет loadCidrs().
        setCidrPage(1);
      }
    } catch (e) {
      message.error(
        e instanceof Error
          ? e.message
          : t("routingPage.cidrAddError"),
      );
    } finally {
      setAddingCidr(false);
    }
  };


  /*
   * --------------------------------------------------------------------------
   * Update domain group
   * --------------------------------------------------------------------------
   */

  const changeDomainGroup = async (
    pattern: string,
    group: number | null,
  ) => {
    if (group === null) {
      return;
    }

    try {
      await updateDomain(
        pattern,
        group,
      );

      setDomains((prev) =>
        prev.map((item) =>
          item.pattern === pattern
            ? {
                ...item,
                group,
              }
            : item,
        ),
      );

      message.success(
        t(
          "routingPage.domainGroupUpdated",
        ),
      );
    } catch (e) {
      message.error(
        e instanceof Error
          ? e.message
          : t(
              "routingPage.domainGroupUpdateError",
            ),
      );
    }
  };
  /* Update default router */
  const changeDefaultRoute = async (
    workerGroup: number,
  ) => {
    setLoadingDefaultRoute(true);

    try {
      const response = await updateDefaultRoute(
        workerGroup,
      );

      setDefaultRoute(response.worker_group);

      message.success(
        t("routingPage.defaultRouteUpdated"),
      );
    } catch (e) {
      message.error(
        e instanceof Error
          ? e.message
          : t("routingPage.defaultRouteUpdateError"),
      );
    } finally {
      setLoadingDefaultRoute(false);
    }
  };

  /*
   * --------------------------------------------------------------------------
   * Update CIDR group
   * --------------------------------------------------------------------------
   */

  const changeCidrGroup = async (
    cidr: string,
    group: number | null,
  ) => {
    if (group === null) {
      return;
    }

    try {
      await updateCidr(
        cidr,
        group,
      );

      setCidrs((prev) =>
        prev.map((item) =>
          item.cidr === cidr
            ? {
                ...item,
                worker_group: group,
              }
            : item,
        ),
      );

      message.success(
        t(
          "routingPage.cidrGroupUpdated",
        ),
      );
    } catch (e) {
      message.error(
        e instanceof Error
          ? e.message
          : t(
              "routingPage.cidrGroupUpdateError",
            ),
      );
    }
  };

  /*
   * --------------------------------------------------------------------------
   * Delete domain
   * --------------------------------------------------------------------------
   */

  const removeDomain = async (
    pattern: string,
  ) => {
    setDeletingDomain(pattern);

    try {
      await deleteDomain(pattern);

      message.success(
        t("routingPage.domainDeleted"),
      );

      /*
       * Не пытаемся вручную удалять из domains.
       * Backend должен быть единственным источником истины.
       *
       * Если удалили последний элемент страницы,
       * переходим на предыдущую страницу.
       */
      if (
        domains.length === 1 &&
        domainPage > 1
      ) {
        setDomainPage(
          (page) => page - 1,
        );
      } else {
        await loadDomains();
      }
    } catch (e) {
      message.error(
        e instanceof Error
          ? e.message
          : t(
              "routingPage.domainDeleteError",
            ),
      );
    } finally {
      setDeletingDomain(null);
    }
  };

  /*
   * --------------------------------------------------------------------------
   * Delete CIDR
   * --------------------------------------------------------------------------
   */

  const removeCidr = async (
    cidr: string,
  ) => {
    setDeletingCidr(cidr);

    try {
      await deleteCidr(cidr);

      message.success(
        t("routingPage.cidrDeleted"),
      );

      if (
        cidrs.length === 1 &&
        cidrPage > 1
      ) {
        setCidrPage(
          (page) => page - 1,
        );
      } else {
        await loadCidrs();
      }
    } catch (e) {
      message.error(
        e instanceof Error
          ? e.message
          : t(
              "routingPage.cidrDeleteError",
            ),
      );
    } finally {
      setDeletingCidr(null);
    }
  };

  /*
   * --------------------------------------------------------------------------
   * Domain columns
   * --------------------------------------------------------------------------
   */

  const domainColumns: TableProps<DomainRule>["columns"] =
    [
      {
        title: t(
          "routingPage.pattern",
        ),
        dataIndex: "pattern",
        key: "pattern",

        render: (
          pattern: string,
        ) => {
          const isWildcard =
            pattern.startsWith(
              "*.",
            );

          return (
            <Space>
              <span>
                {pattern}
              </span>

              {isWildcard && (
                <Tag>
                  {t(
                    "routingPage.subdomains",
                  )}
                </Tag>
              )}
            </Space>
          );
        },
      },

      {
        title: t(
          "groupsPage.groupName",
        ),
        dataIndex: "group",
        key: "group",
        width: 220,

        render: (
          groupId: number | null,
          domain: DomainRule,
        ) => (
          <Select
            value={
              groupId ?? undefined
            }
            placeholder={t(
              "groupsPage.selectGroup",
            )}
            style={{
              minWidth: 160,
            }}
            loading={
              loadingGroups
            }
            options={
              groupOptions
            }
            onChange={(
              value,
            ) =>
              void changeDomainGroup(
                domain.pattern,
                value ?? null,
              )
            }
          />
        ),
      },

      {
        title: t(
          "common.actions",
        ),
        key: "actions",
        width: 80,

        render: (
          _,
          domain,
        ) => (
          <Button
            danger
            icon={
              <DeleteOutlined />
            }
            loading={
              deletingDomain ===
              domain.pattern
            }
            aria-label={t(
              "common.delete",
            )}
            onClick={() =>
              void removeDomain(
                domain.pattern,
              )
            }
          />
        ),
      },
    ];

  /*
   * --------------------------------------------------------------------------
   * CIDR columns
   * --------------------------------------------------------------------------
   */

  const cidrColumns: TableProps<CidrRule>["columns"] =
    [
      {
        title: "CIDR",
        dataIndex: "cidr",
        key: "cidr",

        render: (
          cidr: string,
        ) => (
          <code>
            {cidr}
          </code>
        ),
      },

      {
        title: t(
          "groupsPage.groupName",
        ),
        dataIndex:
          "worker_group",
        key: "worker_group",
        width: 220,

        render: (
          groupId: number | null,
          cidr: CidrRule,
        ) => (
          <Select
            value={
              groupId ?? undefined
            }
            placeholder={t(
              "groupsPage.selectGroup",
            )}
            style={{
              minWidth: 160,
            }}
            loading={
              loadingGroups
            }
            options={
              groupOptions
            }
            onChange={(
              value,
            ) =>
              void changeCidrGroup(
                cidr.cidr,
                value ?? null,
              )
            }
          />
        ),
      },

      {
        title: t(
          "common.actions",
        ),
        key: "actions",
        width: 80,

        render: (
          _,
          cidr,
        ) => (
          <Button
            danger
            icon={
              <DeleteOutlined />
            }
            loading={
              deletingCidr ===
              cidr.cidr
            }
            aria-label={t(
              "common.delete",
            )}
            onClick={() =>
              void removeCidr(
                cidr.cidr,
              )
            }
          />
        ),
      },
    ];

  /*
   * --------------------------------------------------------------------------
   * Domains tab
   * --------------------------------------------------------------------------
   */

  const domainsTab = (
    <div>
      <div
        style={{
          margin: "0px 0 24px",
          padding: 12,
          border: "1px solid #f0f0f0",
          borderRadius: 8,
          background: "white",
        }}
      >
        <Space
          direction="vertical"
          size={12}
          style={{ width: "100%" }}
        >
          <Typography.Text strong>
            {t("routingPage.addDomain")}
          </Typography.Text>
          <Space
            wrap
            size={[12, 12]}
            style={{ width: "100%" }}
          >
            <Input.TextArea
              value={newDomains}
              placeholder={t(
                "routingPage.domainPlaceholder",
              )}
              style={{
                width: 280,
              }}
              onChange={(event) =>
                setNewDomains(event.target.value)
              }
            />

            <Select
              value={domainGroup ?? undefined}
              placeholder={t("groupsPage.selectGroup")}
              style={{
                minWidth: 200,
              }}
              loading={loadingGroups}
              options={groupOptions}
              onChange={(value) =>
                setDomainGroup(value ?? null)
              }
            />

            <Button
              type="primary"
              icon={<PlusOutlined />}
              loading={addingDomain}
              onClick={() => void handleAddDomains()}
            >
              {t("common.add")}
            </Button>
          </Space>

          <Checkbox
            checked={includeSubdomains}
            onChange={(event) =>
              setIncludeSubdomains(event.target.checked)
            }
          >
            {t("routingPage.includeSubdomains")}
          </Checkbox>
        </Space>
      </div>

      <Input
        allowClear
        prefix={
          <SearchOutlined />
        }
        value={
          domainSearchInput
        }
        placeholder={t(
          "routingPage.searchDomain",
        )}
        style={{
          width: 300,
          marginBottom: 16,
        }}
        onChange={(
          event,
        ) =>
          setDomainSearchInput(
            event.target.value,
          )
        }
      />

      <Table<DomainRule>
        rowKey="pattern"
        columns={
          domainColumns
        }
        dataSource={
          domains
        }
        loading={
          loadingDomains
        }
        scroll={{
          x: 600,
        }}
        pagination={{
          current:
            domainPage,
          pageSize:
            domainPageSize,
          total:
            domainTotal,
          showSizeChanger:
            true,

          onChange: (
            page,
            pageSize,
          ) => {
            if (
              pageSize !==
              domainPageSize
            ) {
              setDomainPageSize(
                pageSize,
              );
              setDomainPage(1);
              return;
            }

            setDomainPage(
              page,
            );
          },
        }}
      />
    </div>
  );
  /* Default route tab */
  const defaultRouteTab = (
    <Card
      size="small"
      style={{
        maxWidth: 500,
      }}
    >
      <Space
        direction="vertical"
        size={12}
        style={{
          width: "100%",
        }}
      >
        <Typography.Text>
          {t("routingPage.descriptionRoute")}
        </Typography.Text>

        <Select
          value={defaultRoute ?? undefined}
          loading={updatingDefaultRoute || loadingGroups}
          disabled={updatingDefaultRoute}
          options={defaultRouteOptions}
          style={{
            width: "100%",
          }}
          placeholder={t(
            "routingPage.selectDefaultRoute",
          )}
          onChange={(value) =>
            void changeDefaultRoute(value)
          }
        />
      </Space>
    </Card>
  );

  /*
   * --------------------------------------------------------------------------
   * CIDR tab
   * --------------------------------------------------------------------------
   */

  const cidrsTab = (
    <div>
      <Card
        size="small"
        style={{
          marginBottom: 20,
        }}
      >
        <Space
          direction="vertical"
          size={12}
          style={{
            width: "100%",
          }}
        >
          <Typography.Text strong>
            {t("routingPage.addCidrs")}
          </Typography.Text>

          <Space
            wrap
            size={[12, 12]}
            style={{
              width: "100%",
            }}
          >
            <Input.TextArea
              value={newCidr}
              placeholder="192.168.0.1/24"
              style={{
                width: 260,
              }}
              onChange={(event) =>
                setNewCidr(event.target.value)
              }
            />

            <Select
              value={cidrGroup ?? undefined}
              placeholder={t("groupsPage.selectGroup")}
              style={{
                minWidth: 200,
              }}
              loading={loadingGroups}
              options={groupOptions}
              onChange={(value) =>
                setCidrGroup(value ?? null)
              }
            />

            <Button
              type="primary"
              icon={<PlusOutlined />}
              loading={addingCidr}
              onClick={() =>
                void handleAddCidrs()
              }
            >
              {t("common.add")}
            </Button>
          </Space>
        </Space>
      </Card>

      <Input
        allowClear
        prefix={
          <SearchOutlined />
        }
        value={
          cidrSearchInput
        }
        placeholder={t(
          "routingPage.searchCidr",
        )}
        style={{
          width: 300,
          marginBottom: 16,
        }}
        onChange={(
          event,
        ) =>
          setCidrSearchInput(
            event.target.value,
          )
        }
      />

      <Table<CidrRule>
        rowKey="cidr"
        columns={
          cidrColumns
        }
        dataSource={
          cidrs
        }
        loading={
          loadingCidrs
        }
        scroll={{
          x: 600,
        }}
        pagination={{
          current:
            cidrPage,
          pageSize:
            cidrPageSize,
          total:
            cidrTotal,
          showSizeChanger:
            true,

          onChange: (
            page,
            pageSize,
          ) => {
            if (
              pageSize !==
              cidrPageSize
            ) {
              setCidrPageSize(
                pageSize,
              );
              setCidrPage(1);
              return;
            }

            setCidrPage(
              page,
            );
          },
        }}
      />
    </div>
  );

  /*
   * --------------------------------------------------------------------------
   * Render
   * --------------------------------------------------------------------------
   */

  return (
    <div className="routing-page">
      <div className="page-heading">
        <div>
          <h1>
            {t(
              "routingPage.h1",
            )}
          </h1>
        </div>
      </div>

      <Tabs
        defaultActiveKey={activeTab}
        onChange={(key) => {
          setSearchParams((params) => {
            params.set("tab", key);
            return params;
          });
        }}
        items={[
          {
            key: "default",
            label: t("routingPage.defaultRoute"),
            children: defaultRouteTab,
          },
          {
            key: "domains",
            label: t(
              "routingPage.domains",
            ),
            children:
              domainsTab,
          },
          {
            key: "cidrs",
            label: t(
              "routingPage.cidrs",
            ),
            children:
              cidrsTab,
          },
        ]}
      />
    </div>
  );
}
