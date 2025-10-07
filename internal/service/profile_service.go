package service

import (
	"errors"
	"fmt"
	"strings"

	"github.com/commitlog/internal/db"
	"gorm.io/gorm"
)

var (
	// ErrProfileContactNotFound 在指定的联系信息不存在时返回
	ErrProfileContactNotFound = errors.New("profile contact not found")
	// ErrProfileContactInvalidInput 在输入数据不完整时返回
	ErrProfileContactInvalidInput = errors.New("invalid profile contact input")
)

// ProfileService 负责维护关于我页面的联系信息
// 提供排序、增删改查能力，与 handler 解耦

type ProfileService struct {
	db *gorm.DB
}

// NewProfileService 构造 ProfileService
func NewProfileService(gdb *gorm.DB) *ProfileService {
	return &ProfileService{db: gdb}
}

// ProfileContactInput 描述创建或更新联系信息时可设置的字段
// Sort/Visible 使用指针判断是否显式传入

type ProfileContactInput struct {
	Platform string
	Label    string
	Value    string
	Link     string
	Icon     string
	Sort     *int
	Visible  *bool
}

func platformKey(platform string) string {
	return strings.ToLower(strings.TrimSpace(platform))
}

func defaultIconForPlatform(platform string) string {
	switch platformKey(platform) {
	case "wechat":
		return "wechat"
	case "email":
		return "email"
	case "github":
		return "github"
	case "telegram":
		return "telegram"
	case "x", "twitter":
		return "x"
	case "website":
		return "website"
	default:
		return ""
	}
}

func normalizeUsername(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	return strings.TrimPrefix(trimmed, "@")
}

func platformLink(platform, value, link string) string {
	trimmedLink := strings.TrimSpace(link)
	if trimmedLink != "" {
		return trimmedLink
	}
	trimmedValue := strings.TrimSpace(value)
	switch platformKey(platform) {
	case "email":
		if trimmedValue == "" {
			return ""
		}
		return "mailto:" + trimmedValue
	case "github":
		if trimmedValue == "" {
			return ""
		}
		return fmt.Sprintf("https://github.com/%s", normalizeUsername(trimmedValue))
	case "telegram":
		if trimmedValue == "" {
			return ""
		}
		return fmt.Sprintf("https://t.me/%s", normalizeUsername(trimmedValue))
	case "x", "twitter":
		if trimmedValue == "" {
			return ""
		}
		return fmt.Sprintf("https://x.com/%s", normalizeUsername(trimmedValue))
	case "website":
		return trimmedValue
	default:
		return trimmedLink
	}
}

func applyPlatformDefaults(contact *db.ProfileContact) {
	if contact == nil {
		return
	}
	if strings.TrimSpace(contact.Icon) == "" {
		if icon := defaultIconForPlatform(contact.Platform); icon != "" {
			contact.Icon = icon
		}
	}
	contact.Link = platformLink(contact.Platform, contact.Value, contact.Link)
}

// ListContacts 返回联系信息集合，默认按照排序值升序
// 如果 includeHidden 为 false，则过滤掉 Visible=false 的条目
func (s *ProfileService) ListContacts(includeHidden bool) ([]db.ProfileContact, error) {
	query := s.db.Model(&db.ProfileContact{})
	if !includeHidden {
		query = query.Where("visible = ?", true)
	}

	var items []db.ProfileContact
	if err := query.Order("sort ASC, id ASC").Find(&items).Error; err != nil {
		return nil, fmt.Errorf("list profile contacts: %w", err)
	}

	return items, nil
}

// GetContact 根据主键获取联系信息
func (s *ProfileService) GetContact(id uint) (*db.ProfileContact, error) {
	var item db.ProfileContact
	if err := s.db.First(&item, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrProfileContactNotFound
		}
		return nil, fmt.Errorf("get profile contact: %w", err)
	}
	return &item, nil
}

// CreateContact 新建联系信息，未指定排序时自动追加到末尾
func (s *ProfileService) CreateContact(input ProfileContactInput) (*db.ProfileContact, error) {
	if err := validateProfileContactInput(input); err != nil {
		return nil, err
	}

	sortValue, err := s.resolveSort(input.Sort)
	if err != nil {
		return nil, err
	}

	visible := true
	if input.Visible != nil {
		visible = *input.Visible
	}

	contact := db.ProfileContact{
		Platform: strings.TrimSpace(input.Platform),
		Label:    strings.TrimSpace(input.Label),
		Value:    strings.TrimSpace(input.Value),
		Link:     strings.TrimSpace(input.Link),
		Icon:     strings.TrimSpace(input.Icon),
		Sort:     sortValue,
		Visible:  visible,
	}
	applyPlatformDefaults(&contact)

	if err := s.db.Create(&contact).Error; err != nil {
		return nil, fmt.Errorf("create profile contact: %w", err)
	}

	return &contact, nil
}

// UpdateContact 更新指定联系信息
func (s *ProfileService) UpdateContact(id uint, input ProfileContactInput) (*db.ProfileContact, error) {
	if err := validateProfileContactInput(input); err != nil {
		return nil, err
	}

	var contact db.ProfileContact
	if err := s.db.First(&contact, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrProfileContactNotFound
		}
		return nil, fmt.Errorf("find profile contact: %w", err)
	}

	contact.Platform = strings.TrimSpace(input.Platform)
	contact.Label = strings.TrimSpace(input.Label)
	contact.Value = strings.TrimSpace(input.Value)
	contact.Link = strings.TrimSpace(input.Link)
	contact.Icon = strings.TrimSpace(input.Icon)
	applyPlatformDefaults(&contact)

	if input.Sort != nil {
		contact.Sort = *input.Sort
	}
	if input.Visible != nil {
		contact.Visible = *input.Visible
	}

	if err := s.db.Save(&contact).Error; err != nil {
		return nil, fmt.Errorf("update profile contact: %w", err)
	}

	return &contact, nil
}

// DeleteContact 删除指定联系信息
func (s *ProfileService) DeleteContact(id uint) error {
	if err := s.db.Delete(&db.ProfileContact{}, id).Error; err != nil {
		return fmt.Errorf("delete profile contact: %w", err)
	}
	return nil
}

// ReorderContacts 按给定顺序重排排序字段
// 传入的 IDs 会被依次赋值 0,1,2...，未包含的条目保持原排序
func (s *ProfileService) ReorderContacts(ids []uint) error {
	if len(ids) == 0 {
		return nil
	}

	return s.db.Transaction(func(tx *gorm.DB) error {
		for index, id := range ids {
			if err := tx.Model(&db.ProfileContact{}).Where("id = ?", id).Update("sort", index).Error; err != nil {
				return fmt.Errorf("reorder profile contacts: %w", err)
			}
		}
		return nil
	})
}

func (s *ProfileService) resolveSort(sortPtr *int) (int, error) {
	if sortPtr != nil {
		return *sortPtr, nil
	}

	var maxSort int
	if err := s.db.Model(&db.ProfileContact{}).Select("COALESCE(MAX(sort), -1)").Scan(&maxSort).Error; err != nil {
		return 0, fmt.Errorf("resolve profile contact sort: %w", err)
	}

	return maxSort + 1, nil
}

func validateProfileContactInput(input ProfileContactInput) error {
	platform := strings.TrimSpace(input.Platform)
	if platform == "" {
		return fmt.Errorf("%w: platform is required", ErrProfileContactInvalidInput)
	}
	if strings.TrimSpace(input.Label) == "" {
		return fmt.Errorf("%w: label is required", ErrProfileContactInvalidInput)
	}
	if strings.TrimSpace(input.Value) == "" {
		return fmt.Errorf("%w: value is required", ErrProfileContactInvalidInput)
	}
	if platformKey(platform) == "wechat" && strings.TrimSpace(input.Link) == "" {
		return fmt.Errorf("%w: wechat contact requires qr image", ErrProfileContactInvalidInput)
	}
	return nil
}
