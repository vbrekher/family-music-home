package model

// FamilyChild is backend-only account policy. It is loaded with the canonical
// user on each authentication; changing the visual interface never removes it.
type FamilyChild struct {
	ParentID            string `json:"parentId,omitempty"`
	Interface           string `json:"interface"`
	Density             string `json:"density"`
	StoryLibraries      []int  `json:"storyLibraries"`
	AllowFamilyMessages bool   `json:"allowFamilyMessages"`
	Enabled             bool   `json:"enabled"`
}
