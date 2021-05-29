package constants

const ValidatorID = 810112564787675166

const (
	ActionComment        = "comment"
	ActionApprove        = "approve"
	ActionRequestChanges = "request-changes"
	ActionAccept         = "accept"
	ActionMarkAdded      = "mark-added"
	ActionReject         = "reject"
	ActionUpload         = "upload-file"
)

const (
	RoleAdministrator = "Administrator"
	RoleModerator     = "Moderator"
	RoleCurator       = "Curator"
	RoleHacker        = "Hacker"
	RoleTester        = "Tester"
	RoleVIP           = "VIP"
	RoleArchivist     = "Archivist"
	RoleMechanic      = "Mechanic"
	RoleHunter        = "Hunter"
	TrialCurator      = "Trial Curator"
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
		TrialCurator,
	}
}

// IsStaff returns true if user has any staff role
func IsStaff(roles []string) bool {
	staffRoles := StaffRoles()
	for _, role := range roles {
		for _, staffRole := range staffRoles {
			if role == staffRole {
				return true
			}
		}
	}
	return false
}

const (
	ResourceKeySubmissionID  = "submission-id"
	ResourceKeySubmissionIDs = "submission-ids"
	ResourceKeyFileID        = "file-id"
	ResourceKeyFileIDs       = "file-ids"
)
