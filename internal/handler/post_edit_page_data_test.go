package handler

import (
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/commitlog/internal/db"
	"github.com/gin-gonic/gin"
)

func TestPostEditPageData_LocalizesTagNames(t *testing.T) {
	api, cleanup := setupTestDB(t)
	defer cleanup()

	tag := db.Tag{Name: "产品"}
	if err := db.DB.Create(&tag).Error; err != nil {
		t.Fatalf("seed tag: %v", err)
	}
	if err := db.DB.Create(&db.TagTranslation{TagID: tag.ID, Language: "en", Name: "Product"}).Error; err != nil {
		t.Fatalf("seed translation: %v", err)
	}

	post := db.Post{Content: "# Test\nContent", Status: "draft", UserID: 1}
	if err := db.DB.Create(&post).Error; err != nil {
		t.Fatalf("seed post: %v", err)
	}
	if err := db.DB.Model(&post).Association("Tags").Append(&tag); err != nil {
		t.Fatalf("associate tag: %v", err)
	}

	req := httptest.NewRequest("GET", "/admin/posts/"+strconv.Itoa(int(post.ID))+"?lang=en", nil)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	c.Params = gin.Params{gin.Param{Key: "id", Value: strconv.Itoa(int(post.ID))}}

	data := api.postEditPageData(c)

	allTags, ok := data["allTags"].([]gin.H)
	if !ok {
		t.Fatalf("expected allTags to be []gin.H, got %T", data["allTags"])
	}
	if len(allTags) != 1 {
		t.Fatalf("expected 1 tag, got %d", len(allTags))
	}
	if allTags[0]["Name"] != "Product" {
		t.Fatalf("expected allTags[0].Name to be %q, got %v", "Product", allTags[0]["Name"])
	}

	postValue, ok := data["post"].(*db.Post)
	if !ok {
		t.Fatalf("expected post to be *db.Post, got %T", data["post"])
	}
	if len(postValue.Tags) != 1 {
		t.Fatalf("expected post to have 1 tag, got %d", len(postValue.Tags))
	}
	if postValue.Tags[0].Name != "Product" {
		t.Fatalf("expected post tag name to be %q, got %q", "Product", postValue.Tags[0].Name)
	}
}
