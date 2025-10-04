import { ChangeEvent, FormEvent, useState } from "react";

interface Props {
  title: string;
  description: string;
  buttonLabel?: string;
  disabled?: boolean;
  onSubmit: (payload: Record<string, string>) => Promise<void>;
  fields: Array<{
    name: string;
    label: string;
    placeholder?: string;
    type?: string;
    required?: boolean;
  }>;
}

export const RpcActionCard = ({ title, description, buttonLabel = "Submit", disabled, onSubmit, fields }: Props) => {
  const [values, setValues] = useState<Record<string, string>>(() => Object.fromEntries(fields.map((field) => [field.name, ""])));
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);

  const handleSubmit = async (evt: FormEvent<HTMLFormElement>) => {
    evt.preventDefault();
    setLoading(true);
    setError(null);
    setSuccess(null);
    try {
      await onSubmit(values);
      setSuccess("Action completed");
      setValues(Object.fromEntries(fields.map((field) => [field.name, ""])));
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="rounded-xl border border-slate-800 bg-slate-950/40 p-5">
      <h3 className="text-sm font-semibold uppercase tracking-wide text-cyan">{title}</h3>
      <p className="mt-1 text-xs text-slate-400">{description}</p>
      <form className="mt-4 space-y-3" onSubmit={handleSubmit}>
        {fields.map((field) => (
          <div key={field.name}>
            <label className="text-[11px] font-semibold uppercase tracking-wide text-slate-400" htmlFor={field.name}>
              {field.label}
            </label>
            <input
              id={field.name}
              type={field.type ?? "text"}
              value={values[field.name] ?? ""}
              required={field.required}
              placeholder={field.placeholder}
              onChange={(evt: ChangeEvent<HTMLInputElement>) =>
                setValues((prev: Record<string, string>) => ({ ...prev, [field.name]: evt.target.value }))}
              className="mt-1 w-full rounded-md border border-slate-800 bg-slate-900 px-3 py-2 text-sm text-slate-100 focus:border-sky focus:outline-none focus:ring-2 focus:ring-sky"
            />
          </div>
        ))}
        {error ? <p className="text-sm text-rose-400">{error}</p> : null}
        {success ? <p className="text-sm text-emerald-400">{success}</p> : null}
        <button
          type="submit"
          disabled={disabled || loading}
          className="w-full rounded-md bg-cyan px-4 py-2 text-xs font-semibold uppercase tracking-wide text-slate-900 transition hover:bg-sky disabled:opacity-60"
        >
          {loading ? "Processing" : buttonLabel}
        </button>
      </form>
    </div>
  );
};
