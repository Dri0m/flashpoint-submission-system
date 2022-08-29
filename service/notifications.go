package service

import (
	"context"
	"fmt"
	"github.com/Dri0m/flashpoint-submission-system/constants"
	"github.com/Dri0m/flashpoint-submission-system/database"
	"github.com/Dri0m/flashpoint-submission-system/types"
	"github.com/Dri0m/flashpoint-submission-system/utils"
	"strings"
	"time"
)

// createNotification formats and stores notification
func (s *SiteService) createNotification(dbs database.DBSession, authorID, sid int64, action string) error {
	validAction := false
	for _, a := range constants.GetActionsWithNotification() {
		if action == a {
			validAction = true
			break
		}
	}
	if !validAction {
		return nil
	}

	mentionUserIDs, err := s.dal.GetUsersForNotification(dbs, authorID, sid, action)
	if err != nil {
		utils.LogCtx(dbs.Ctx()).Error(err)
		return err
	}

	if len(mentionUserIDs) == 0 {
		return nil
	}

	var b strings.Builder
	b.WriteString("You've got mail!\n")
	b.WriteString(fmt.Sprintf("<https://fpfss.unstable.life/web/submission/%d>\n", sid))

	if action == constants.ActionComment {
		b.WriteString("There is a new comment on the submission.")
	} else if action == constants.ActionApprove {
		b.WriteString("The submission has been approved.")
	} else if action == constants.ActionRequestChanges {
		b.WriteString("User has requested changes on the submission.")
	} else if action == constants.ActionMarkAdded {
		b.WriteString("The submission has been marked as added to Flashpoint.")
	} else if action == constants.ActionUpload {
		b.WriteString(fmt.Sprintf("A new version has been uploaded by <@%d>", authorID))
	} else if action == constants.ActionReject {
		b.WriteString("The submission has been rejected.")
	}
	b.WriteString("\n")

	for _, userID := range mentionUserIDs {
		b.WriteString(fmt.Sprintf(" <@%d>", userID))
	}

	b.WriteString("\n----------------------------------------------------------\n")
	msg := b.String()

	if err := s.dal.StoreNotification(dbs, msg, constants.NotificationDefault); err != nil {
		utils.LogCtx(dbs.Ctx()).Error(err)
		return dberr(err)
	}

	return nil
}

// createCurationFeedMessage formats and stores message for the curation feed
func (s *SiteService) createCurationFeedMessage(dbs database.DBSession, authorID, sid int64, isSubmissionNew, isCurationValid bool, meta *types.CurationMeta, isAudition bool) error {
	var b strings.Builder

	if isSubmissionNew {
		b.WriteString(fmt.Sprintf("A new submission has been uploaded by <@%d>\n", authorID))
	} else {
		b.WriteString(fmt.Sprintf("A submission update has been uploaded by <@%d>\n", authorID))
	}
	b.WriteString(fmt.Sprintf("<https://fpfss.unstable.life/web/submission/%d>\n", sid))

	if !isCurationValid {
		b.WriteString("Unfortunately, it does not quite reach the quality required to satisfy the cool crab.\n")
	}

	if meta.Library != nil && meta.Platform != nil && meta.Title != nil && meta.Extreme != nil {
		llib := strings.ToLower(*meta.Library)
		if strings.Contains(llib, "arcade") {
			b.WriteString("🎮")
		} else if strings.Contains(llib, "theatre") {
			b.WriteString("🎞️")
		} else {
			b.WriteString("❓")
		}

		b.WriteString(" ")

		// TODO save this in map
		lplat := strings.ToLower(*meta.Platform)
		if strings.Contains(lplat, "3d groove") {
			b.WriteString("<:3DGroove:569691574276063242>")
		} else if strings.Contains(lplat, "eva") {
			b.WriteString("<:EVA:936449221446492212>")
		} else if strings.Contains(lplat, "3dvia player") {
			b.WriteString("<:3DVIA_Player:496151464784166946>")
		} else if strings.Contains(lplat, "axel player") {
			b.WriteString("<:AXEL_Player:813079894267265094>")
		} else if strings.Contains(lplat, "activex") {
			b.WriteString("<:ActiveX:699093212949643365>")
		} else if strings.Contains(lplat, "atmosphere") {
			b.WriteString("<:Atmosphere:781105689002901524>")
		} else if strings.Contains(lplat, "authorware") {
			b.WriteString("<:Authorware:582105144410243073>")
		} else if strings.Contains(lplat, "burster") {
			b.WriteString("<:Burster:743995494736461854>")
		} else if strings.Contains(lplat, "cult3d") {
			b.WriteString("<:Cult3D:806277196473040896>")
		} else if strings.Contains(lplat, "deepv") {
			b.WriteString("<:DeepV:812079774843142255>")
		} else if strings.Contains(lplat, "flash") {
			b.WriteString("<:Flash:750823911326875648>")
		} else if strings.Contains(lplat, "gobit") {
			b.WriteString("<:GoBit:629511736608686080>")
		} else if strings.Contains(lplat, "html5") {
			b.WriteString("<:HTML5:701930562746712142>")
		} else if strings.Contains(lplat, "hyper-g") {
			b.WriteString("<:HyperG:817543962088570880>")
		} else if strings.Contains(lplat, "hypercosm") {
			b.WriteString("<:Hypercosm:814623525038063697>")
		} else if strings.Contains(lplat, "java") {
			b.WriteString("<:Java:482697866377297920>")
		} else if strings.Contains(lplat, "livemath") {
			b.WriteString("<:LiveMath_Plugin:808999958043951104>")
		} else if strings.Contains(lplat, "octree view") {
			b.WriteString("<:Octree_View:809147835927756831>")
		} else if strings.Contains(lplat, "play3d") {
			b.WriteString("<:Play3D:812079775152734209>")
		} else if strings.Contains(lplat, "popcap plugin") {
			b.WriteString("<:PopCap:604433459179552798>")
		} else if strings.Contains(lplat, "protoplay") {
			b.WriteString("<:ProtoPlay:806614012829761587>")
		} else if strings.Contains(lplat, "pulse") {
			b.WriteString("<:Pulse:720682372982505472>")
		} else if strings.Contains(lplat, "rebol") {
			b.WriteString("<:REBOL:806995243085987862>")
		} else if strings.Contains(lplat, "shiva3d") {
			b.WriteString("<:ShiVa3d:643124144812326934>")
		} else if strings.Contains(lplat, "shockwave") {
			b.WriteString("<:Shockwave:727436274625019965>")
		} else if strings.Contains(lplat, "silverlight") {
			b.WriteString("<:Silverlight:492112373625257994>")
		} else if strings.Contains(lplat, "tcl") {
			b.WriteString("<:Tcl:737419431067779144>")
		} else if strings.Contains(lplat, "unity") {
			b.WriteString("<:Unity:600478910169481216>")
		} else if strings.Contains(lplat, "vrml") {
			b.WriteString("<:VRML:737049432817664070>")
		} else if strings.Contains(lplat, "viscape") {
			b.WriteString("<:Viscape:814623877039652886>")
		} else if strings.Contains(lplat, "vitalize") {
			b.WriteString("<:Vitalize:700924839912800332>")
		} else if strings.Contains(lplat, "xara plugin") {
			b.WriteString("<:Xara_Plugin:807439131768258561>")
		} else if strings.Contains(lplat, "alambik") {
			b.WriteString("<:Alambik:814621713350262856>")
		} else if strings.Contains(lplat, "animaflex") {
			b.WriteString("<:AnimaFlex:807016001618968596>")
		} else if strings.Contains(lplat, "webmap") {
			b.WriteString("<:Visual_WebMap:815055929589891122>")
		} else if strings.Contains(lplat, "bitplayer") {
			b.WriteString("<:BitPlayer:793866776684658708>")
		} else if strings.Contains(lplat, "o2c") {
			b.WriteString("<:o2c:864618351538733117>")
		} else if strings.Contains(lplat, "freehand") {
			b.WriteString("<:FreeHand:872557242854035487>")
		} else if strings.Contains(lplat, "hotsauce") {
			b.WriteString("<:HotSauce:866419306451173416>")
		} else if strings.Contains(lplat, "thingviewer") {
			b.WriteString("<:ThingViewer:872565939068084254>")
		} else if strings.Contains(lplat, "dpgraph") {
			b.WriteString("<:DPGraph:879995725595934720>")
		} else if strings.Contains(lplat, "envoy") {
			b.WriteString("<:Envoy:880973750013673492>")
		} else if strings.Contains(lplat, "pixound") {
			b.WriteString("<:Pixound:881324002482745425>")
		} else if strings.Contains(lplat, "show it") {
			b.WriteString("<:ShowIt:887139518652772442>")
		} else if strings.Contains(lplat, "mhsv") {
			b.WriteString("<:MHSV:909580737068560445>")
		} else if strings.Contains(lplat, "squeak") {
			b.WriteString("<:Squeak:933419800384925767>")
		} else if strings.Contains(lplat, "pointplus") {
			b.WriteString("<:PointPlus:917230760337997834>")
		} else if strings.Contains(lplat, "calendar quick") {
			b.WriteString("<:Calendar_Quick:917575719536697424>")
		} else if strings.Contains(lplat, "e-animator") {
			b.WriteString("<:e_animator:933419945931448421>")
		} else if strings.Contains(lplat, "flatland rover") {
			b.WriteString("<:Flatland_Rover:936449386005819453>")
		} else if strings.Contains(lplat, "dfusion") {
			b.WriteString("<:DFusion:953097421779501056>")
		} else if strings.Contains(lplat, "webanimator") {
			b.WriteString("<:WebAnimator:953095732896874598>")
		} else if strings.Contains(lplat, "harvard webshow") {
			b.WriteString("<:HarvardWebShow:957708182376054794>")
		} else if strings.Contains(lplat, "svf viewer") {
			b.WriteString("<:SVFviewer:957708220569366560>")
		} else if strings.Contains(lplat, "surround video") {
			b.WriteString("<:SurroundVideo:957719709153919016>")
		} else if strings.Contains(lplat, "formula one") {
			b.WriteString("<:FormulaOne:962052882285330532>")
		} else if strings.Contains(lplat, "illuminatus") {
			b.WriteString("<:Illuminatus:962052900023050324>")
		} else if strings.Contains(lplat, "asap webshow") {
			b.WriteString("<:ASAPWebShow:962766908837474404>")
		} else if strings.Contains(lplat, "lightning strike") {
			b.WriteString("<:LightningStrike:962766923936981012>")
		} else if strings.Contains(lplat, "smoothmove panorama") {
			b.WriteString("<:SmoothMovePanorama:962766936570208386>")
		} else if strings.Contains(lplat, "ambulant") {
			b.WriteString("<:Ambulant:963972260413186129>")
		} else if strings.Contains(lplat, "ipix") {
			b.WriteString("<:iPix:964160323336679514>")
		} else if strings.Contains(lplat, "jcamp-dx") {
			b.WriteString("<:JCAMPDX:964914642491154452>")
		} else if strings.Contains(lplat, "abouttime") {
			b.WriteString("<:AboutTime:965282823361687572>")
		} else if strings.Contains(lplat, "aboutpeople") {
			b.WriteString("<:AboutPeople:965282823110000671>")
		} else if strings.Contains(lplat, "live picture viewer") {
			b.WriteString("<:LivePicture:965670969643503739>")
		} else if strings.Contains(lplat, "x3d") {
			b.WriteString("<:X3D:966206271910969374>")
		} else if strings.Contains(lplat, "noteworthy composer") {
			b.WriteString("<:NoteWorthyComposer:967141915189477407>")
		} else if strings.Contains(lplat, "mapguide") {
			b.WriteString("<:MapGuide:968518302580215879>")
		} else if strings.Contains(lplat, "blender") {
			b.WriteString("<:Blender:968940112627003463>")
		} else if strings.Contains(lplat, "vream") {
			b.WriteString("<:VReam:972878890190131260>")
		} else if strings.Contains(lplat, "common ground") {
			b.WriteString("<:CommonGround:973082691375333446>")
		} else if strings.Contains(lplat, "jutvision") {
			b.WriteString("<:jutvision:973274204063555635>")
		} else if strings.Contains(lplat, "cool 360") {
			b.WriteString("<:cool360:973967480370368612>")
		} else if strings.Contains(lplat, "mrsid") {
			b.WriteString("<:MrSID:976488638600847420>")
		} else if strings.Contains(lplat, "panoramix") {
			b.WriteString("<:PanoramIX:976488559836037150>")
		} else if strings.Contains(lplat, "mbed") {
			b.WriteString("<:MBed:976501234636841080>")
		} else if strings.Contains(lplat, "djvu") {
			b.WriteString("<:DjVu:984885288700620800>")
		} else if strings.Contains(lplat, "jamagic") {
			b.WriteString("<:Jamagic:988401673401675797>")
		} else if strings.Contains(lplat, "scorch") {
			b.WriteString("<:Scorch:990511328160526346>")
		} else if strings.Contains(lplat, "petz player") {
			b.WriteString("<:Petz:1010910107044937729>")
		} else if strings.Contains(lplat, "sizzler") {
			b.WriteString("<:Sizzler:1010910145540268073>")
		} else {
			b.WriteString("❓")
		}

		b.WriteString(" ")

		if *meta.Extreme == "Yes" {
			b.WriteString("<:extreme:778145279714918400>")
		}

		b.WriteString(" ")

		b.WriteString(*meta.Title)
		b.WriteString("\n")
	}

	// also notify all those that want to know about new audition uploads
	if isAudition {
		auditionMentionUserIDs, err := s.dal.GetUsersForUniversalNotification(dbs, authorID, constants.ActionAuditionUpload)
		if err != nil {
			utils.LogCtx(dbs.Ctx()).Error(err)
			return err
		}

		for _, uid := range auditionMentionUserIDs {
			b.WriteString(fmt.Sprintf("<@%d> ", uid))
		}
		b.WriteString("\n")
	}

	b.WriteString("----------------------------------------------------------\n")
	msg := b.String()

	if err := s.dal.StoreNotification(dbs, msg, constants.NotificationCurationFeed); err != nil {
		return err
	}

	return nil
}

// createDeletionNotification formats and stores deletion notification
func (s *SiteService) createDeletionNotification(dbs database.DBSession, authorID, deleterID int64, sid, cid, fid *int64, reason string) error {
	if sid == nil {
		utils.LogCtx(dbs.Ctx()).Panic("submission id cannot be nil")
	}
	if cid != nil && fid != nil {
		utils.LogCtx(dbs.Ctx()).Panic("both cid and fid provided - not valid")
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("You've got mail! <@%d>\n", authorID))
	b.WriteString(fmt.Sprintf("<https://fpfss.unstable.life/web/submission/%d>\n", *sid))
	if cid != nil {
		b.WriteString(fmt.Sprintf("Your comment #%d was deleted by <@%d>\n", *cid, deleterID))
	} else if fid != nil {
		b.WriteString(fmt.Sprintf("Your file #%d was deleted by <@%d>\n", *fid, deleterID))
	} else {
		b.WriteString(fmt.Sprintf("Your submission #%d was deleted by <@%d>\n", *sid, deleterID))
	}
	b.WriteString(fmt.Sprintf("Reason: %s", reason))
	b.WriteString("\n----------------------------------------------------------\n")
	msg := b.String()

	if err := s.dal.StoreNotification(dbs, msg, constants.NotificationDefault); err != nil {
		utils.LogCtx(dbs.Ctx()).Error(err)
		return dberr(err)
	}

	return nil
}

// ProduceRemindersAboutRequestedChanges generates notifications for every user with submissions which are waiting for changes more than a month
func (s *SiteService) ProduceRemindersAboutRequestedChanges(ctx context.Context) (int, error) {
	dbs, err := s.dal.NewSession(ctx)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return 0, dberr(err)
	}
	defer dbs.Rollback()

	ongoing := "ongoing"
	results, _, err := s.dal.SearchSubmissions(dbs, &types.SubmissionsFilter{RequestedChangedStatus: &ongoing, DistinctActionsNot: []string{"mark-added", "reject"}})
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return 0, dberr(err)
	}

	authors := make(map[int64]int)
	for _, r := range results {
		if !r.UpdatedAt.Before(time.Now().Add(-time.Hour * 24 * 30)) {
			continue
		}

		author := r.SubmitterID

		if _, ok := authors[author]; !ok {
			authors[author] = 1
		} else {
			authors[author] += 1
		}
	}

	for authorID, count := range authors {
		var b strings.Builder
		b.WriteString(fmt.Sprintf("You've got mail! <@%d>\n", authorID))
		b.WriteString(fmt.Sprintf("You've got %d submissions with changes requested for more than a month\n", count))
		b.WriteString(fmt.Sprintf("You should visit <https://fpfss.unstable.life/web/submissions?filter-layout=advanced&submitter-id=%d&requested-changes-status=ongoing&distinct-action-not=mark-added&asc-desc=asc&order-by=updated> and decide what to do about them.\n", authorID))
		b.WriteString("\n----------------------------------------------------------\n")
		msg := b.String()

		if err := s.dal.StoreNotification(dbs, msg, constants.NotificationDefault); err != nil {
			utils.LogCtx(dbs.Ctx()).Error(err)
			return 0, dberr(err)
		}
	}

	if err := dbs.Commit(); err != nil {
		utils.LogCtx(ctx).Error(err)
		return 0, dberr(err)
	}

	return len(authors), nil
}
