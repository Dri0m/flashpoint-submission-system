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

// IsTrialCurator returns true if user is trial curator
func IsTrialCurator(roles []string) bool {
	trialCuratorRoles := TrialCuratorRoles()
	for _, role := range roles {
		for _, trialCuratorRole := range trialCuratorRoles {
			if role == trialCuratorRole {
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
