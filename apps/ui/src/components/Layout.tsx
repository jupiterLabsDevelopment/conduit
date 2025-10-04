import { Fragment } from "react";
import { Link, Navigate, Outlet, useLocation } from "react-router-dom";

import { useAuth } from "../context/AuthContext";

export const Layout = () => {
  const { user, logout } = useAuth();
  const location = useLocation();

  if (!user) {
    return <Navigate to="/login" replace state={{ from: location }} />;
  }

  const isOwner = user.role === "owner";

  return (
    <div className="min-h-screen bg-navy text-slate-100">
      <header className="border-b border-slate-800 bg-slate-950/80 backdrop-blur">
        <div className="mx-auto flex max-w-6xl items-center justify-between px-6 py-4">
          <div className="flex items-center gap-8">
            <Link to="/servers" className="text-lg font-semibold tracking-tight text-cyan hover:text-sky">
              Conduit
            </Link>
            <nav className="hidden gap-6 text-sm font-medium md:flex">
              <Link to="/servers" className="hover:text-sky">
                Servers
              </Link>
              {isOwner ? (
                <Link to="/api-keys" className="hover:text-sky">
                  API keys
                </Link>
              ) : null}
            </nav>
          </div>
          <div className="flex items-center gap-4 text-sm">
            <Fragment>
              <div className="text-slate-300">
                <span className="font-medium text-sky">{user.email}</span>
                <span className="ml-2 rounded-full bg-slate-800 px-2 py-0.5 text-xs uppercase tracking-wide text-cyan">
                  {user.role}
                </span>
              </div>
              <button
                type="button"
                onClick={() => {
                  void logout();
                }}
                className="rounded-md bg-slate-800 px-3 py-1.5 text-xs font-semibold uppercase tracking-wide text-slate-200 transition hover:bg-slate-700"
              >
                Sign out
              </button>
            </Fragment>
          </div>
        </div>
      </header>
      <main className="mx-auto max-w-6xl px-6 py-10">
        <Outlet />
      </main>
    </div>
  );
};
