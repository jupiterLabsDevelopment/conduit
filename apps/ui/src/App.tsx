import { BrowserRouter, Navigate, Route, Routes } from "react-router-dom";

import { useAuth } from "./context/AuthContext";
import { Layout } from "./components/Layout";
import LoginPage from "./pages/LoginPage";
import ServersPage from "./pages/ServersPage";
import ServerDetailPage from "./pages/ServerDetailPage";
import ApiKeysPage from "./pages/ApiKeysPage";

const App = () => {
  const { loading, user } = useAuth();

  if (loading) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-navy text-slate-100">
        <div className="text-lg font-semibold tracking-wide">Loading Conduit…</div>
      </div>
    );
  }

  return (
    <BrowserRouter>
      <Routes>
        <Route path="/login" element={user ? <Navigate to="/servers" replace /> : <LoginPage />} />
        <Route element={<Layout />}>
          <Route index element={<Navigate to="/servers" replace />} />
          <Route path="/servers" element={<ServersPage />} />
          <Route path="/servers/:id" element={<ServerDetailPage />} />
          <Route path="/api-keys" element={<ApiKeysPage />} />
        </Route>
        <Route path="*" element={<Navigate to={user ? "/servers" : "/login"} replace />} />
      </Routes>
    </BrowserRouter>
  );
};

export default App;
