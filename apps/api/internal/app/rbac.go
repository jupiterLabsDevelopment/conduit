package app

import "strings"

type rbacRule struct {
	prefix string
	role   Role
}

var rbacRules = []rbacRule{
	{prefix: "minecraft:server/stop", role: RoleOwner},
	{prefix: "minecraft:server/save", role: RoleModerator},
	{prefix: "minecraft:server/system_message", role: RoleModerator},
	{prefix: "minecraft:server/status", role: RoleViewer},
	{prefix: "minecraft:players/", role: RoleModerator},
	{prefix: "minecraft:players", role: RoleViewer},
	{prefix: "minecraft:gamerules/update", role: RoleModerator},
	{prefix: "minecraft:gamerules", role: RoleViewer},
	{prefix: "minecraft:serversettings/", role: RoleModerator},
	{prefix: "minecraft:allowlist/", role: RoleModerator},
	{prefix: "minecraft:allowlist", role: RoleViewer},
	{prefix: "minecraft:operators/", role: RoleModerator},
	{prefix: "minecraft:operators", role: RoleModerator},
	{prefix: "minecraft:bans/", role: RoleModerator},
	{prefix: "minecraft:bans", role: RoleModerator},
	{prefix: "minecraft:ip_bans/", role: RoleModerator},
	{prefix: "minecraft:ip_bans", role: RoleModerator},
}

func roleForMethod(method string) Role {
	if method == "" {
		return RoleViewer
	}
	for _, rule := range rbacRules {
		if strings.HasPrefix(method, rule.prefix) {
			return rule.role
		}
	}
	return RoleOwner
}
