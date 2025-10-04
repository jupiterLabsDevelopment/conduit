import { FormEvent, useState } from "react";
import { useLocation, useNavigate } from "react-router-dom";

import { useAuth } from "../context/AuthContext";

const LoginPage = () => {
  const { login, bootstrap } = useAuth();
  const navigate = useNavigate();
  const location = useLocation();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [showBootstrap, setShowBootstrap] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  const redirectAfterAuth = () => {
    const state = location.state as { from?: Location } | undefined;
    const dest = state?.from?.pathname ?? "/servers";
    navigate(dest, { replace: true });
  };

  const handleSubmit = async (evt: FormEvent<HTMLFormElement>) => {
    evt.preventDefault();
    setLoading(true);
    setError(null);
    try {
      await login(email, password);
      redirectAfterAuth();
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setLoading(false);
    }
  };

  const handleBootstrap = async () => {
    setLoading(true);
    setError(null);
    try {
      await bootstrap(email, password);
      await login(email, password);
      redirectAfterAuth();
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="flex min-h-screen items-center justify-center bg-navy px-6 text-slate-100">
      <div className="w-full max-w-md rounded-2xl border border-slate-800 bg-slate-950/80 p-8 shadow-xl shadow-slate-950/30">
        <h1 className="text-2xl font-semibold text-cyan">Conduit Admin</h1>
        <p className="mt-1 text-sm text-slate-400">Secure control channel for Minecraft servers.</p>

        <form className="mt-8 space-y-6" onSubmit={handleSubmit}>
          <div>
            <label htmlFor="email" className="text-xs font-semibold uppercase tracking-wide text-slate-400">
              Email
            </label>
            <input
              id="email"
              type="email"
              autoComplete="email"
              required
              value={email}
              onChange={(evt) => setEmail(evt.target.value)}
              className="mt-1 w-full rounded-md border border-slate-800 bg-slate-900 px-3 py-2 text-sm text-slate-100 focus:border-sky focus:outline-none focus:ring-2 focus:ring-sky"
            />
          </div>

          <div>
            <label htmlFor="password" className="text-xs font-semibold uppercase tracking-wide text-slate-400">
              Password
            </label>
            <input
              id="password"
              type="password"
              autoComplete="current-password"
              required
              value={password}
              onChange={(evt) => setPassword(evt.target.value)}
              className="mt-1 w-full rounded-md border border-slate-800 bg-slate-900 px-3 py-2 text-sm text-slate-100 focus:border-sky focus:outline-none focus:ring-2 focus:ring-sky"
            />
          </div>

          {error ? <p className="text-sm text-rose-400">{error}</p> : null}

          <button
            type="submit"
            disabled={loading}
            className="w-full rounded-md bg-cyan px-4 py-2 text-sm font-semibold uppercase tracking-wide text-slate-900 transition hover:bg-sky disabled:opacity-60"
          >
            {loading ? "Signing in" : "Sign in"}
          </button>
        </form>

        <div className="mt-6 border-t border-slate-800 pt-6 text-xs text-slate-400">
          <button
            type="button"
            className="font-semibold text-sky hover:text-cyan"
            onClick={() => setShowBootstrap((prev) => !prev)}
          >
            {showBootstrap ? "Hide bootstrap instructions" : "First run? Bootstrap owner account"}
          </button>
          {showBootstrap ? (
            <div className="mt-3 space-y-3">
              <p>
                Use the same credentials above and click <span className="font-semibold text-cyan">Bootstrap owner</span> to create the
                first owner account.
              </p>
              <button
                type="button"
                onClick={handleBootstrap}
                disabled={loading}
                className="w-full rounded-md border border-slate-700 px-4 py-2 text-xs font-semibold uppercase tracking-wide text-slate-100 hover:border-sky"
              >
                {loading ? "Bootstrapping" : "Bootstrap owner"}
              </button>
            </div>
          ) : null}
        </div>
      </div>
    </div>
  );
};

export default LoginPage;
