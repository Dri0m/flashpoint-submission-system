package types

type CurationMeta struct {
	SubmissionID        int64
	SubmissionFileID    int64
	ApplicationPath     *string `json:"Application Path"`
	Developer           *string `json:"Developer"`
	Extreme             *string `json:"Extreme"`
	GameNotes           *string `json:"Game Notes"`
	Languages           *string `json:"Languages"`
	LaunchCommand       *string `json:"Launch Command"`
	OriginalDescription *string `json:"Original Description"`
	PlayMode            *string `json:"Play Mode"`
	Platform            *string `json:"Platform"`
	Publisher           *string `json:"Publisher"`
	ReleaseDate         *string `json:"Release Date"`
	Series              *string `json:"Series"`
	Source              *string `json:"Source"`
	Status              *string `json:"Status"`
	Tags                *string `json:"Tags"`
	TagCategories       *string `json:"Tag Categories"`
	Title               *string `json:"Title"`
	AlternateTitles     *string `json:"Alternate Title"`
	Library             *string `json:"Library"`
	Version             *string `json:"Version"`
	CurationNotes       *string `json:"Curation Notes"`
	MountParameters     *string `json:"Mount Parameters"`
	//AdditionalApplications *CurationFormatAddApps `json:"Additional Applications"`
}
