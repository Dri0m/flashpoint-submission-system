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
	RoleTrialEditor   = "Trial Editor"
	RoleTheBlue       = "The Blue"
	RoleTheD          = "The D"
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
		RoleTheBlue,
		RoleTheD,
	}
}

func TrialCuratorRoles() []string {
	return []string{
		RoleTrialCurator,
	}
}

func TrialEditorRoles() []string {
	return []string{
		RoleTrialEditor,
	}
}

func DeleterRoles() []string {
	return []string{
		RoleAdministrator,
		RoleModerator,
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
		RoleModerator,
		RoleAdministrator,
	}
}

func GodRoles() []string {
	return []string{
		RoleTheBlue,
		RoleTheD,
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

func isTrialEditor(roles []string) bool {
	return HasAnyRole(roles, TrialEditorRoles())
}

// IsDeleter allows users to soft delete things
func IsDeleter(roles []string) bool {
	return HasAnyRole(roles, DeleterRoles())
}

// IsDecider allows user to decide the state of submissions (approve, request changes, accept, reject)
func IsDecider(roles []string) bool {
	return HasAnyRole(roles, DeciderRoles())
}

// IsAdder allows user to mark submission as added to flashpoint
func IsAdder(roles []string) bool {
	return HasAnyRole(roles, AdderRoles())
}

// IsGod allows user to do various things
func IsGod(roles []string) bool {
	return HasAnyRole(roles, GodRoles())
}

func IsGodOrColin(roles []string, uid int64) bool {
	return HasAnyRole(roles, GodRoles()) || uid == 689080719460663414
}
