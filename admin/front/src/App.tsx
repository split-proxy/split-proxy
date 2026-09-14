import { Navigate, Route, Routes } from "react-router-dom";
import { ProtectedRoute } from "./components/ProtectedRoute";
import { AdminLayout } from "./components/AdminLayout";
import { HomePage } from "./pages/HomePage";
import { LoginPage } from "./pages/LoginPage";
import { UserFormPage } from "./pages/UserFormPage";
import { UsersPage } from "./pages/UsersPage";
import { ProxiesPage } from "./pages/proxy/ProxiesPage";
import { ProxyFormPage } from "./pages/proxy/NewProxyFormPage";
import { GroupsPage } from "./pages/group/GroupsPage";
import { GroupFormPage } from "./pages/group/NewGroupFormPage";
import { SplittingPage } from "./pages/splitting/SplittingPage";
import { ConfigProvider } from "./shared/config/ConfigContext";

export default function App() {
  return (
    <ConfigProvider>
      <Routes>
        <Route path="/login" element={<LoginPage />} />

        <Route element={<ProtectedRoute />}>
          <Route path="/" element={<AdminLayout />}>
            <Route index element={<HomePage />} />
            <Route path="users" element={<UsersPage />} />
            <Route path="users/new" element={<UserFormPage />} />
            <Route path="users/:id/edit" element={<UserFormPage />} />

            <Route path="proxies" element={<ProxiesPage/>} />
            <Route path="proxies/new" element={<ProxyFormPage />} />
            <Route path="proxies/:id/edit" element={<ProxyFormPage />} />

            <Route path="groups" element={<GroupsPage/>} />
            <Route path="groups/new" element={<GroupFormPage />} />
            <Route path="groups/:id/edit" element={<GroupFormPage />} />

            <Route path="splitting" element={<SplittingPage/>} />
          </Route>
        </Route>

        <Route path="*" element={<Navigate to="/users" replace />} />
      </Routes>
    </ConfigProvider>
  );
}
