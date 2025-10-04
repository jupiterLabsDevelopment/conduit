package app

import "strings"

type rbacRule struct {
	prefix string
	role   Role
}

var rbacRules = []rbacRule{
	{prefix: "minecraft:server/stop", role: RoleOwner},
	{prefix: "minecraft:server/save", role: RoleModerator},
	{prefix: "minecraft:allowlist/", role: RoleModerator},
	{prefix: "minecraft:operators/", role: RoleModerator},
	{prefix: "minecraft:gamerule/", role: RoleModerator},
	{prefix: "minecraft:settings/", role: RoleModerator},
	{prefix: "minecraft:players/list", role: RoleViewer},
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
