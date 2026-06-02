import { optionalApp } from "./wails";

export type AuthType = "password" | "private_key" | "ssh_agent";

export type ConnectionProfile = {
  id: string;
  name: string;
  host: string;
  port: number;
  username: string;
  authType: AuthType;
  keyPath?: string;
  description?: string;
  passwordSaved?: boolean;
};

export async function loadProfiles(): Promise<ConnectionProfile[]> {
  const service = optionalApp();
  if (!service) {
    return [];
  }
  return service.LoadProfiles();
}

export async function saveProfiles(profiles: ConnectionProfile[]): Promise<void> {
  const service = optionalApp();
  if (!service) {
    return;
  }
  return service.SaveProfiles(profiles);
}

export async function saveProfile(
  profile: ConnectionProfile,
  password: string,
  savePassword: boolean,
): Promise<ConnectionProfile | undefined> {
  const service = optionalApp();
  if (!service) {
    return undefined;
  }
  return service.SaveProfile(profile, password, savePassword);
}

export async function deleteProfile(profileId: string): Promise<void> {
  const service = optionalApp();
  if (!service) {
    return;
  }
  return service.DeleteProfile(profileId);
}
