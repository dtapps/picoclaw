import { launcherFetch } from "@/api/http"

export interface EnvVarEntry {
  key: string
  value: string
  enabled: boolean
  sensitive: boolean
  note: string
}

export interface EnvVarsConfig {
  variables: EnvVarEntry[]
  env_file: string
}

async function request<T>(url: string, init?: RequestInit): Promise<T> {
  const res = await launcherFetch(url, {
    headers: {
      "Content-Type": "application/json",
    },
    ...init,
  })
  if (!res.ok) {
    const text = await res.text()
    throw new Error(`Request failed: ${res.status} ${text}`)
  }
  return res.json() as Promise<T>
}

export async function getEnvVarsConfig(): Promise<EnvVarsConfig> {
  return request<EnvVarsConfig>("/api/env-vars")
}

export async function updateEnvVarsConfig(config: EnvVarsConfig): Promise<void> {
  const res = await launcherFetch("/api/env-vars", {
    method: "PUT",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify(config),
  })
  if (!res.ok) {
    const text = await res.text()
    throw new Error(`Update failed: ${res.status} ${text}`)
  }
}

export async function importEnvFile(file: File): Promise<EnvVarEntry[]> {
  const formData = new FormData()
  formData.append("file", file)

  const res = await launcherFetch("/api/env-vars/import", {
    method: "POST",
    body: formData,
  })
  if (!res.ok) {
    const text = await res.text()
    throw new Error(`Import failed: ${res.status} ${text}`)
  }
  return res.json() as Promise<EnvVarEntry[]>
}

export async function exportEnvFile(): Promise<Blob> {
  const res = await launcherFetch("/api/env-vars/export")
  if (!res.ok) {
    throw new Error(`Export failed: ${res.status} ${res.statusText}`)
  }
  return res.blob()
}
