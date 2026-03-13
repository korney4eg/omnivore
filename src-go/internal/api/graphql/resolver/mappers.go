package resolver

import (
	"encoding/json"

	"github.com/omnivore-app/omnivore/internal/api/graphql/model"
	"github.com/omnivore-app/omnivore/internal/api/graphql/scalar"
	dbmodel "github.com/omnivore-app/omnivore/internal/api/models"
)

// — User / Profile / Personalization —

func mapUser(u *dbmodel.User) *model.User {
	if u == nil {
		return nil
	}
	gql := &model.User{
		ID:        u.ID,
		Name:      u.Name,
		Email:     &u.Email,
		Source:    &u.Source,
		CreatedAt: scalar.Date(u.CreatedAt),
		// sharedArticles required by schema; return empty slice
		SharedArticles: []*model.FeedArticle{},
	}
	if u.Profile != nil {
		gql.Profile = mapProfile(u.Profile)
		gql.Picture = u.Profile.PictureURL
	}
	return gql
}

func mapProfile(p *dbmodel.UserProfile) *model.Profile {
	if p == nil {
		return nil
	}
	return &model.Profile{
		ID:         p.ID,
		Username:   p.Username,
		Private:    p.Private,
		Bio:        p.Bio,
		PictureURL: p.PictureURL,
	}
}

func mapPersonalization(p *dbmodel.UserPersonalization) *model.UserPersonalization {
	if p == nil {
		return &model.UserPersonalization{}
	}
	gql := &model.UserPersonalization{
		ID:                   &p.ID,
		Theme:                p.Theme,
		FontSize:             p.FontSize,
		FontFamily:           p.FontFamily,
		Margin:               p.Margin,
		LibraryLayoutType:    p.LibraryLayoutType,
		SpeechVoice:          p.SpeechVoice,
		SpeechSecondaryVoice: p.SpeechSecondaryVoice,
		SpeechRate:           p.SpeechRate,
		SpeechVolume:         p.SpeechVolume,
	}
	if p.LibrarySortOrder != nil {
		v := model.SortOrder(*p.LibrarySortOrder)
		gql.LibrarySortOrder = &v
	}
	if len(p.Fields) > 0 {
		if b, err := json.Marshal(p.Fields); err == nil {
			gql.Fields = scalar.JSON(b)
		}
	}
	return gql
}

// — LibraryItem → Article —

func mapLibraryItem(item *dbmodel.LibraryItem) *model.Article {
	if item == nil {
		return nil
	}
	state := model.ArticleSavingRequestStatus(item.State)
	cr := model.ContentReader(item.ContentReader)
	progressTop := float64(item.ReadingProgressTopPercent)
	gql := &model.Article{
		ID:                         item.ID,
		Title:                      item.Title,
		Slug:                       item.Slug,
		URL:                        item.OriginalURL,
		Hash:                       item.ID, // hash not stored separately
		Content:                    item.ReadableContent,
		ContentReader:              cr,
		Author:                     item.Author,
		Description:                item.Description,
		CreatedAt:                  scalar.Date(item.CreatedAt),
		SavedAt:                    scalar.Date(item.SavedAt),
		ReadingProgressPercent:     float64(item.ReadingProgressBottomPercent),
		ReadingProgressTopPercent:  &progressTop,
		ReadingProgressAnchorIndex: item.ReadingProgressLastReadAnchor,
		IsArchived:                 item.ArchivedAt != nil,
		Folder:                     item.Folder,
		SiteName:                   item.SiteName,
		SiteIcon:                   item.SiteIcon,
		Subscription:               item.Subscription,
		Language:                   item.ItemLanguage,
		WordsCount:                 item.WordCount,
		UploadFileID:               item.UploadFileID,
		FeedContent:                item.FeedContent,
		State:                      &state,
		// Slices must be non-nil
		Highlights:      []*model.Highlight{},
		Recommendations: []*model.Recommendation{},
	}
	if item.UpdatedAt != (item.UpdatedAt) {
		d := scalar.Date(item.UpdatedAt)
		gql.UpdatedAt = &d
	}
	if item.PublishedAt != nil {
		d := scalar.Date(*item.PublishedAt)
		gql.PublishedAt = &d
	}
	if item.ReadAt != nil {
		d := scalar.Date(*item.ReadAt)
		gql.ReadAt = &d
	}
	if item.Thumbnail != nil {
		gql.Image = item.Thumbnail
	}
	// Map labels
	for i := range item.Labels {
		gql.Labels = append(gql.Labels, mapLabel(&item.Labels[i]))
	}
	// Map highlights
	for i := range item.Highlights {
		gql.Highlights = append(gql.Highlights, mapHighlight(&item.Highlights[i]))
	}
	return gql
}

func mapLibraryItems(items []dbmodel.LibraryItem) []*model.Article {
	out := make([]*model.Article, 0, len(items))
	for i := range items {
		out = append(out, mapLibraryItem(&items[i]))
	}
	return out
}

// — Label —

func mapLabel(l *dbmodel.Label) *model.Label {
	if l == nil {
		return nil
	}
	createdAt := scalar.Date(l.CreatedAt)
	return &model.Label{
		ID:          l.ID,
		Name:        l.Name,
		Color:       l.Color,
		Description: l.Description,
		CreatedAt:   &createdAt,
	}
}

func mapLabels(labels []dbmodel.Label) []*model.Label {
	out := make([]*model.Label, 0, len(labels))
	for i := range labels {
		out = append(out, mapLabel(&labels[i]))
	}
	return out
}

// — Highlight —

func mapHighlight(h *dbmodel.Highlight) *model.Highlight {
	if h == nil {
		return nil
	}
	createdAt := scalar.Date(h.CreatedAt)
	gql := &model.Highlight{
		ID:        h.ID,
		ShortID:   h.ShortID,
		Quote:     h.Quote,
		Prefix:    h.Prefix,
		Suffix:    h.Suffix,
		Patch:     h.Patch,
		Annotation: h.Annotation,
		Color:     h.Color,
		HTML:      h.HTML,
		CreatedAt: createdAt,
		Type:           model.HighlightType(h.HighlightType),
		Representation: model.RepresentationType(h.Representation),
		// Required slices
		Replies:   []*model.HighlightReply{},
		Reactions: []*model.Reaction{},
		Labels:    []*model.Label{},
	}
	if !h.UpdatedAt.IsZero() {
		d := scalar.Date(h.UpdatedAt)
		gql.UpdatedAt = &d
	}
	if h.SharedAt != nil {
		d := scalar.Date(*h.SharedAt)
		gql.SharedAt = &d
	}
	for i := range h.Labels {
		gql.Labels = append(gql.Labels, mapLabel(&h.Labels[i]))
	}
	return gql
}

func mapHighlights(hs []dbmodel.Highlight) []*model.Highlight {
	out := make([]*model.Highlight, 0, len(hs))
	for i := range hs {
		out = append(out, mapHighlight(&hs[i]))
	}
	return out
}

// — APIKey —

func mapAPIKey(k *dbmodel.APIKey, rawKey *string) *model.APIKey {
	if k == nil {
		return nil
	}
	createdAt := scalar.Date(k.CreatedAt)
	expiresAt := scalar.Date(k.ExpiresAt)
	gql := &model.APIKey{
		ID:        k.ID,
		Name:      k.Name,
		Scopes:    []string(k.Scopes),
		CreatedAt: createdAt,
		ExpiresAt: expiresAt,
		Key:       rawKey,
	}
	if k.UsedAt != nil {
		d := scalar.Date(*k.UsedAt)
		gql.UsedAt = &d
	}
	return gql
}

// — Integration —

func mapIntegration(i *dbmodel.Integration) *model.Integration {
	if i == nil {
		return nil
	}
	createdAt := scalar.Date(i.CreatedAt)
	gql := &model.Integration{
		ID:        i.ID,
		Name:      i.Name,
		Type:      model.IntegrationType(i.Type),
		Token:     i.Token,
		Enabled:   i.Enabled,
		CreatedAt: createdAt,
	}
	if i.UpdatedAt != i.CreatedAt {
		d := scalar.Date(i.UpdatedAt)
		gql.UpdatedAt = &d
	}
	if i.ImportItemState != nil {
		gql.TaskName = i.ImportItemState
	}
	if len(i.Settings) > 0 {
		if b, err := json.Marshal(i.Settings); err == nil {
			gql.Settings = scalar.JSON(b)
		}
	}
	return gql
}

func mapIntegrations(items []dbmodel.Integration) []*model.Integration {
	out := make([]*model.Integration, 0, len(items))
	for i := range items {
		out = append(out, mapIntegration(&items[i]))
	}
	return out
}

// — Webhook —

func mapWebhook(w *dbmodel.Webhook) *model.Webhook {
	if w == nil {
		return nil
	}
	createdAt := scalar.Date(w.CreatedAt)
	events := make([]model.WebhookEvent, 0, len(w.EventTypes))
	for _, e := range w.EventTypes {
		events = append(events, model.WebhookEvent(e))
	}
	gql := &model.Webhook{
		ID:          w.ID,
		URL:         w.URL,
		EventTypes:  events,
		ContentType: w.ContentType,
		Method:      w.Method,
		Enabled:     w.Enabled,
		CreatedAt:   createdAt,
	}
	if w.UpdatedAt != w.CreatedAt {
		d := scalar.Date(w.UpdatedAt)
		gql.UpdatedAt = &d
	}
	return gql
}

func mapWebhooks(ws []dbmodel.Webhook) []*model.Webhook {
	out := make([]*model.Webhook, 0, len(ws))
	for i := range ws {
		out = append(out, mapWebhook(&ws[i]))
	}
	return out
}

// — Rule —

func mapRule(r *dbmodel.Rule) *model.Rule {
	if r == nil {
		return nil
	}
	createdAt := scalar.Date(r.CreatedAt)
	events := make([]model.RuleEventType, 0, len(r.EventTypes))
	for _, e := range r.EventTypes {
		events = append(events, model.RuleEventType(e))
	}
	// Deserialize actions from JSONB
	type rawAction struct {
		Type   string   `json:"type"`
		Params []string `json:"params"`
	}
	var rawActions []rawAction
	if len(r.Actions) > 0 {
		_ = json.Unmarshal([]byte(r.Actions), &rawActions)
	}
	actions := make([]*model.RuleAction, 0, len(rawActions))
	for _, a := range rawActions {
		params := a.Params
		if params == nil {
			params = []string{}
		}
		actions = append(actions, &model.RuleAction{
			Type:   model.RuleActionType(a.Type),
			Params: params,
		})
	}
	gql := &model.Rule{
		ID:         r.ID,
		Name:       r.Name,
		Filter:     r.Filter,
		Actions:    actions,
		Enabled:    r.Enabled,
		EventTypes: events,
		CreatedAt:  createdAt,
	}
	if r.UpdatedAt != r.CreatedAt {
		d := scalar.Date(r.UpdatedAt)
		gql.UpdatedAt = &d
	}
	if r.FailedAt != nil {
		d := scalar.Date(*r.FailedAt)
		gql.FailedAt = &d
	}
	return gql
}

func mapRules(rs []dbmodel.Rule) []*model.Rule {
	out := make([]*model.Rule, 0, len(rs))
	for i := range rs {
		out = append(out, mapRule(&rs[i]))
	}
	return out
}

// — Filter —

func mapFilter(f *dbmodel.Filter) *model.Filter {
	if f == nil {
		return nil
	}
	createdAt := scalar.Date(f.CreatedAt)
	updatedAt := scalar.Date(f.UpdatedAt)
	category := f.Category
	visible := f.Visible
	defaultFilter := f.DefaultFilter
	return &model.Filter{
		ID:            f.ID,
		Name:          f.Name,
		Filter:        f.Filter,
		Position:      f.Position,
		Folder:        f.Folder,
		Description:   f.Description,
		CreatedAt:     createdAt,
		UpdatedAt:     &updatedAt,
		Category:      &category,
		Visible:       &visible,
		DefaultFilter: &defaultFilter,
	}
}

func mapFilters(fs []dbmodel.Filter) []*model.Filter {
	out := make([]*model.Filter, 0, len(fs))
	for i := range fs {
		out = append(out, mapFilter(&fs[i]))
	}
	return out
}

// — Subscription —

func mapSubscription(s *dbmodel.Subscription) *model.Subscription {
	if s == nil {
		return nil
	}
	createdAt := scalar.Date(s.CreatedAt)
	updatedAt := scalar.Date(s.UpdatedAt)
	folder := ""
	if s.Folder != nil {
		folder = *s.Folder
	}
	gql := &model.Subscription{
		ID:               s.ID,
		Name:             s.Name,
		URL:              s.URL,
		Description:      s.Description,
		Status:           model.SubscriptionStatus(s.Status),
		Type:             model.SubscriptionType(s.Type),
		Count:            s.Count,
		Icon:             s.Icon,
		IsPrivate:        &s.IsPrivate,
		AutoAddToLibrary: s.AutoAddToLibrary,
		FetchContent:     s.FetchContent,
		FetchContentType: model.FetchContentType(s.FetchContentType),
		Folder:           folder,
		CreatedAt:        createdAt,
		UpdatedAt:        &updatedAt,
	}
	if s.RefreshedAt != nil {
		d := scalar.Date(*s.RefreshedAt)
		gql.RefreshedAt = &d
	}
	if s.FailedAt != nil {
		d := scalar.Date(*s.FailedAt)
		gql.FailedAt = &d
	}
	return gql
}

func mapSubscriptions(subs []dbmodel.Subscription) []*model.Subscription {
	out := make([]*model.Subscription, 0, len(subs))
	for i := range subs {
		out = append(out, mapSubscription(&subs[i]))
	}
	return out
}

// — NewsletterEmail —

func mapNewsletterEmail(e *dbmodel.NewsletterEmail) *model.NewsletterEmail {
	if e == nil {
		return nil
	}
	folder := ""
	if e.Folder != nil {
		folder = *e.Folder
	}
	return &model.NewsletterEmail{
		ID:               e.ID,
		Address:          e.Address,
		ConfirmationCode: e.ConfirmationCode,
		CreatedAt:        scalar.Date(e.CreatedAt),
		Folder:           folder,
		Name:             e.Name,
		Description:      e.Description,
	}
}

func mapNewsletterEmails(emails []dbmodel.NewsletterEmail) []*model.NewsletterEmail {
	out := make([]*model.NewsletterEmail, 0, len(emails))
	for i := range emails {
		out = append(out, mapNewsletterEmail(&emails[i]))
	}
	return out
}

// — SearchItem —

func mapSearchItem(item *dbmodel.LibraryItem) *model.SearchItem {
	if item == nil {
		return nil
	}
	state := model.ArticleSavingRequestStatus(item.State)
	cr := model.ContentReader(item.ContentReader)
	pt := model.PageType(item.ItemType)
	progressTop := float64(item.ReadingProgressTopPercent)
	si := &model.SearchItem{
		ID:                         item.ID,
		Title:                      item.Title,
		Slug:                       item.Slug,
		URL:                        item.OriginalURL,
		PageType:                   pt,
		ContentReader:              cr,
		CreatedAt:                  scalar.Date(item.CreatedAt),
		IsArchived:                 item.ArchivedAt != nil,
		ReadingProgressPercent:     float64(item.ReadingProgressBottomPercent),
		ReadingProgressTopPercent:  &progressTop,
		ReadingProgressAnchorIndex: item.ReadingProgressLastReadAnchor,
		Author:                     item.Author,
		Image:                      item.Thumbnail,
		Description:                item.Description,
		State:                      &state,
		SiteName:                   item.SiteName,
		SiteIcon:                   item.SiteIcon,
		Subscription:               item.Subscription,
		Language:                   item.ItemLanguage,
		WordsCount:                 item.WordCount,
		UploadFileID:               item.UploadFileID,
		FeedContent:                item.FeedContent,
		Folder:                     item.Folder,
		SavedAt:                    scalar.Date(item.SavedAt),
		Highlights:                 []*model.Highlight{},
		Recommendations:            []*model.Recommendation{},
	}
	if !item.UpdatedAt.IsZero() {
		d := scalar.Date(item.UpdatedAt)
		si.UpdatedAt = &d
	}
	if item.PublishedAt != nil {
		d := scalar.Date(*item.PublishedAt)
		si.PublishedAt = &d
	}
	if item.ReadAt != nil {
		d := scalar.Date(*item.ReadAt)
		si.ReadAt = &d
	}
	if item.ArchivedAt != nil {
		d := scalar.Date(*item.ArchivedAt)
		si.ArchivedAt = &d
	}
	for i := range item.Labels {
		si.Labels = append(si.Labels, mapLabel(&item.Labels[i]))
	}
	for i := range item.Highlights {
		si.Highlights = append(si.Highlights, mapHighlight(&item.Highlights[i]))
	}
	return si
}

func mapSearchItems(items []dbmodel.LibraryItem) []*model.SearchItem {
	out := make([]*model.SearchItem, 0, len(items))
	for i := range items {
		out = append(out, mapSearchItem(&items[i]))
	}
	return out
}
