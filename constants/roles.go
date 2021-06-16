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

func DeciderRoles() []string {
	return []string{
		RoleCurator,
		RoleTester,
	}
}

func AdderRoles() []string {
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

// IsInAudit allows users to access to submit one curation and interact only with it
func IsInAudit(roles []string) bool {
	return !(IsStaff(roles) || IsTrialCurator(roles))
}

// IsStaff allows users to access and interact with all submissions, to a degree
func IsStaff(roles []string) bool {
	return HasAnyRole(roles, StaffRoles())
}

// IsTrialCurator allows user to submit submissions and see only his own submissions
func IsTrialCurator(roles []string) bool {
	return HasAnyRole(roles, TrialCuratorRoles())
}

// IsDeletor allows users to soft delete things
func IsDeletor(roles []string) bool {
	return HasAnyRole(roles, DeletorRoles())
}

// IsDecider allows user to decide the state of submissions (approve, request changes, accept, reject)
func IsDecider(roles []string) bool {
	return HasAnyRole(roles, DeciderRoles())
}

// IsAdder allows user to mark submission as added to flashpoint
func IsAdder(roles []string) bool {
	return HasAnyRole(roles, AdderRoles())
}
