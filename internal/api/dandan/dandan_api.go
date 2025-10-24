package dandan

import (
	"danmu-tool/internal/api"
	"danmu-tool/internal/utils"
	"net/http"

	"github.com/go-chi/chi/v5"
)

var dandanLogger = utils.GetComponentLogger("dandan-api")

type Comment struct {
	Count    int64 `json:"count"`
	Comments []struct {
		CID string `json:"cid"`
		P   string `json:"p"`
		M   string `json:"m"`
	} `json:"comments"`
}

func CommentHandler(w http.ResponseWriter, r *http.Request) {

	token := chi.URLParam(r, "token")
	id := chi.URLParam(r, "id")

	query := r.URL.Query()
	query.Get("from")        // int64
	query.Get("withRelated") // bool
	query.Get("chConvert")   // bool

	dandanLogger.Info("comment api requested", "token", token, "id", id)

	comment := Comment{
		Count: 100,
		Comments: make([]struct {
			CID string `json:"cid"`
			P   string `json:"p"`
			M   string `json:"m"`
		}, 0),
	}

	api.ResponseJSON(w, http.StatusOK, comment)
}

func MatchHandler(w http.ResponseWriter, r *http.Request) {

	//{
	//  "fileName": "天穗之咲稻姬 S01E01",
	//  "fileSize": 0,
	//  "matchMode": "fileNameOnly",
	//  "videoDuration": 0,
	//  "fileHash": "123d05841b9456ccc7420b3f0bb21c3b"
	//}

	//{
	//  "success": true,
	//  "errorCode": 0,
	//  "errorMessage": "",
	//  "isMatched": true,
	//  "matches": [
	//    {
	//      "episodeId": 25000007010001,
	//      "animeId": 7,
	//      "animeTitle": "天穗之咲稻姬",
	//      "episodeTitle": "第1话 天界的咲稻姬",
	//      "type": "tvseries",
	//      "typeDescription": "TV动画",
	//      "shift": 0
	//    }
	//  ]
	//}

	api.ResponseJSON(w, http.StatusOK, "")
}
