package danmaku

type MediaService interface {
	Media(id string) ([]*Media, error)

	Scraper
}
