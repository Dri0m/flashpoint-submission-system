package service

import (
	"github.com/Dri0m/flashpoint-submission-system/constants"
	"github.com/Dri0m/flashpoint-submission-system/types"
	"testing"
)

func Test_isActionValidForSubmission(t *testing.T) {
	type args struct {
		uid        int64
		formAction string
		submission *types.ExtendedSubmission
	}

	var commenterID int64 = 1
	var lastUploaderID int64 = 2

	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "uploader cannot assign for testing",
			args: args{
				uid:        lastUploaderID,
				formAction: constants.ActionAssignTesting,
				submission: &types.ExtendedSubmission{LastUploaderID: lastUploaderID},
			},
			wantErr: true,
		},
		{
			name: "uploader cannot assign for verification",
			args: args{
				uid:        lastUploaderID,
				formAction: constants.ActionAssignVerification,
				submission: &types.ExtendedSubmission{LastUploaderID: lastUploaderID},
			},
			wantErr: true,
		},
		{
			name: "uploader cannot approve",
			args: args{
				uid:        lastUploaderID,
				formAction: constants.ActionApprove,
				submission: &types.ExtendedSubmission{LastUploaderID: lastUploaderID},
			},
			wantErr: true,
		},
		{
			name: "uploader cannot request changes",
			args: args{
				uid:        lastUploaderID,
				formAction: constants.ActionRequestChanges,
				submission: &types.ExtendedSubmission{LastUploaderID: lastUploaderID},
			},
			wantErr: true,
		},
		{
			name: "uploader cannot verify",
			args: args{
				uid:        lastUploaderID,
				formAction: constants.ActionVerify,
				submission: &types.ExtendedSubmission{LastUploaderID: lastUploaderID},
			},
			wantErr: true,
		},
		{
			name: "user cannot double assign for testing",
			args: args{
				uid:        commenterID,
				formAction: constants.ActionAssignTesting,
				submission: &types.ExtendedSubmission{AssignedTestingUserIDs: []int64{commenterID}},
			},
			wantErr: true,
		},
		{
			name: "user cannot double unassign for testing",
			args: args{
				uid:        commenterID,
				formAction: constants.ActionUnassignTesting,
				submission: &types.ExtendedSubmission{},
			},
			wantErr: true,
		},
		{
			name: "user cannot double assign for verification",
			args: args{
				uid:        commenterID,
				formAction: constants.ActionAssignVerification,
				submission: &types.ExtendedSubmission{AssignedVerificationUserIDs: []int64{commenterID}},
			},
			wantErr: true,
		},
		{
			name: "user cannot double unassign for verification",
			args: args{
				uid:        commenterID,
				formAction: constants.ActionUnassignVerification,
				submission: &types.ExtendedSubmission{},
			},
			wantErr: true,
		},
		{
			name: "user cannot double approve",
			args: args{
				uid:        commenterID,
				formAction: constants.ActionApprove,
				submission: &types.ExtendedSubmission{ApprovedUserIDs: []int64{commenterID}},
			},
			wantErr: true,
		},
		{
			name: "user can double request changes",
			args: args{
				uid:        commenterID,
				formAction: constants.ActionRequestChanges,
				submission: &types.ExtendedSubmission{RequestedChangesUserIDs: []int64{commenterID}},
			},
			wantErr: false,
		},
		{
			name: "user cannot assign for testing that's marked as added",
			args: args{
				uid:        commenterID,
				formAction: constants.ActionAssignTesting,
				submission: &types.ExtendedSubmission{DistinctActions: []string{constants.ActionMarkAdded}},
			},
			wantErr: true,
		},
		{
			name: "user cannot unassign for testing that's marked as added",
			args: args{
				uid:        commenterID,
				formAction: constants.ActionUnassignTesting,
				submission: &types.ExtendedSubmission{DistinctActions: []string{constants.ActionMarkAdded}},
			},
			wantErr: true,
		},
		{
			name: "user cannot assign for verification that's marked as added",
			args: args{
				uid:        commenterID,
				formAction: constants.ActionAssignVerification,
				submission: &types.ExtendedSubmission{DistinctActions: []string{constants.ActionMarkAdded}},
			},
			wantErr: true,
		},
		{
			name: "user cannot unassign for verification that's marked as added",
			args: args{
				uid:        commenterID,
				formAction: constants.ActionUnassignVerification,
				submission: &types.ExtendedSubmission{DistinctActions: []string{constants.ActionMarkAdded}},
			},
			wantErr: true,
		},
		{
			name: "user cannot request changes that's marked as added",
			args: args{
				uid:        commenterID,
				formAction: constants.ActionRequestChanges,
				submission: &types.ExtendedSubmission{DistinctActions: []string{constants.ActionMarkAdded}},
			},
			wantErr: true,
		},
		{
			name: "user cannot approve that's marked as added",
			args: args{
				uid:        commenterID,
				formAction: constants.ActionApprove,
				submission: &types.ExtendedSubmission{DistinctActions: []string{constants.ActionMarkAdded}},
			},
			wantErr: true,
		},
		{
			name: "user cannot verify that's marked as added",
			args: args{
				uid:        commenterID,
				formAction: constants.ActionVerify,
				submission: &types.ExtendedSubmission{DistinctActions: []string{constants.ActionMarkAdded}},
			},
			wantErr: true,
		},
		{
			name: "user cannot assign for both at once 1",
			args: args{
				uid:        commenterID,
				formAction: constants.ActionAssignTesting,
				submission: &types.ExtendedSubmission{AssignedVerificationUserIDs: []int64{commenterID}},
			},
			wantErr: true,
		},
		{
			name: "user cannot assign for both at once 2",
			args: args{
				uid:        commenterID,
				formAction: constants.ActionAssignVerification,
				submission: &types.ExtendedSubmission{AssignedTestingUserIDs: []int64{commenterID}},
			},
			wantErr: true,
		},
		{
			name: "user cannot assign for verification if he tested",
			args: args{
				uid:        commenterID,
				formAction: constants.ActionAssignVerification,
				submission: &types.ExtendedSubmission{AssignedTestingUserIDs: []int64{commenterID}},
			},
			wantErr: true,
		},
		{
			name: "user cannot approve if he is not assigned for testing",
			args: args{
				uid:        commenterID,
				formAction: constants.ActionApprove,
				submission: &types.ExtendedSubmission{},
			},
			wantErr: true,
		},
		{
			name: "user cannot verify if he is not assigned for verification",
			args: args{
				uid:        commenterID,
				formAction: constants.ActionApprove,
				submission: &types.ExtendedSubmission{},
			},
			wantErr: true,
		},
		{
			name: "user cannot mark as added if it is not verified",
			args: args{
				uid:        commenterID,
				formAction: constants.ActionMarkAdded,
				submission: &types.ExtendedSubmission{},
			},
			wantErr: true,
		},
		{
			name: "user cannot assign for testing if they approved",
			args: args{
				uid:        commenterID,
				formAction: constants.ActionAssignTesting,
				submission: &types.ExtendedSubmission{ApprovedUserIDs: []int64{commenterID}},
			},
			wantErr: true,
		},
		{
			name: "user cannot assign for verification if they verified",
			args: args{
				uid:        commenterID,
				formAction: constants.ActionAssignTesting,
				submission: &types.ExtendedSubmission{VerifiedUserIDs: []int64{commenterID}},
			},
			wantErr: true,
		},
		{
			name: "user cannot assign for testing if they verified",
			args: args{
				uid:        commenterID,
				formAction: constants.ActionAssignTesting,
				submission: &types.ExtendedSubmission{VerifiedUserIDs: []int64{commenterID}},
			},
			wantErr: true,
		},
		{
			name: "user cannot assign for verification if they approved",
			args: args{
				uid:        commenterID,
				formAction: constants.ActionAssignVerification,
				submission: &types.ExtendedSubmission{ApprovedUserIDs: []int64{commenterID}},
			},
			wantErr: true,
		},
		{
			name: "user cannot reject submission that's marked as added to flashpoint",
			args: args{
				uid:        commenterID,
				formAction: constants.ActionReject,
				submission: &types.ExtendedSubmission{DistinctActions: []string{constants.ActionMarkAdded}},
			},
			wantErr: true,
		},
		{
			name: "user cannot reject submission that's already rejected",
			args: args{
				uid:        commenterID,
				formAction: constants.ActionReject,
				submission: &types.ExtendedSubmission{DistinctActions: []string{constants.ActionReject}},
			},
			wantErr: true,
		},
		{
			name: "user cannot upload a submission that's already rejected",
			args: args{
				uid:        commenterID,
				formAction: constants.ActionUpload,
				submission: &types.ExtendedSubmission{DistinctActions: []string{constants.ActionReject}},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := isActionValidForSubmission(tt.args.uid, tt.args.formAction, tt.args.submission); (err != nil) != tt.wantErr {
				t.Errorf("isActionValidForSubmission() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
