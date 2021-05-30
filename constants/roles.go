package constants

const (
	RoleAdministrator = "Administrator"
	RoleModerator     = "Moderator"
	RoleCurator       = "Curator"
	RoleHacker        = "Hacker"
	RoleTester        = "Tester"
	RoleArchivist     = "Archivist"
	RoleMechanic      = "Mechanic"
	RoleHunter        = "Hunter"
	RoleTrialCurator  = "Trial Curator"
)

func StaffRoles() []string {
	return []string{
		RoleAdministrator,
		RoleModerator,
		RoleCurator,
		RoleHacker,
		RoleTester,
		RoleArchivist,
		RoleMechanic,
		RoleHunter,
	}
}

func TrialCuratorRoles() []string {
	return []string{
		RoleTrialCurator,
	}
}

func DeletorRoles() []string {
	return []string{
		RoleAdministrator,
	}
}

func HasAnyRole(has, needs []string) bool {
	for _, role := range has {
		for _, neededRole := range needs {
			if role == neededRole {
				return true
			}
		}
	}
	return false
}

// IsStaff returns true if user has any staff role
func IsStaff(roles []string) bool {
	return HasAnyRole(roles, StaffRoles())
}

// IsTrialCurator returns true if user is trial curator
func IsTrialCurator(roles []string) bool {
	return HasAnyRole(roles, TrialCuratorRoles())
}

// IsDeletor returns true if user is trial curator
func IsDeletor(roles []string) bool {
	return HasAnyRole(roles, DeletorRoles())
}
