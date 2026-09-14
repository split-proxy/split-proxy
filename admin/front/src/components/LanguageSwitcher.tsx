import { GlobalOutlined } from "@ant-design/icons";
import { Select } from "antd";
import { useTranslation } from "react-i18next";

export function LanguageSwitcher() {
  const { i18n } = useTranslation();

  const changeLanguage = async (language: string) => {
    await i18n.changeLanguage(language);
    localStorage.setItem("language", language);
  };

  return (
    <Select
      value={i18n.language}
      onChange={changeLanguage}
      suffixIcon={<GlobalOutlined />}
      options={[
        {
          value: "ru",
          label: "RU",
        },
        {
          value: "en",
          label: "EN",
        },
      ]}
    />
  );
}
