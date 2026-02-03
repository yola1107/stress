package data

import (
	"context"
	"time"

	"stress/internal/biz/member"
)

// Member 数据库实体
type Member struct {
	ID            int64   `xorm:"pk autoincr 'id'"`
	MemberName    string  `xorm:"'member_name'"`
	Password      string  `xorm:"'password'"`
	NickName      string  `xorm:"'nick_name'"`
	Balance       float64 `xorm:"'balance'"`
	Currency      string  `xorm:"'currency'"`
	State         int64   `xorm:"'state'"`
	LastLoginTime int64   `xorm:"'last_login_time'"`
	IP            string  `xorm:"'ip'"`
	MerchantID    int64   `xorm:"'merchant_id'"`
	Merchant      string  `xorm:"'merchant'"`
	Remark        string  `xorm:"'remark'"`
	IsDelete      int64   `xorm:"'is_delete'"`
	MemberType    int64   `xorm:"'member_type'"`
	TrueName      string  `xorm:"'true_name'"`
	TelPrefix     string  `xorm:"'tel_prefix'"`
	VipLevel      int64   `xorm:"'vip_level'"`
	Phone         string  `xorm:"'phone'"`
	ParentID      string  `xorm:"'parent_id'"`
	Email         string  `xorm:"'email'"`
	CreatedAt     int64   `xorm:"'created_at'"`
	UpdatedAt     int64   `xorm:"'updated_at'"`
	Version       int64   `xorm:"'version'"`
}

func (m *Member) TableName() string {
	return "member"
}

// inChunkSize MySQL 预处理占位符有限制，IN 子句分批避免 "too many placeholders"
const inChunkSize = 250

// BatchUpsertMembers 表中已有该账号（member_name）则只回填 ID 不插入；没有则插入新行并回填 ID
func (r *dataRepo) BatchUpsertMembers(ctx context.Context, members []member.Info) error {
	if len(members) == 0 {
		return nil
	}

	session := r.data.db.NewSession().Context(ctx)
	defer session.Close()

	if err := session.Begin(); err != nil {
		return err
	}

	// 1. 批量查已有：按 member_name 查，已有则只回填 ID，不插入
	names := make([]string, len(members))
	for i := range members {
		names[i] = members[i].Name
	}
	existingByName := make(map[string]int64)
	for i := 0; i < len(names); i += inChunkSize {
		end := i + inChunkSize
		if end > len(names) {
			end = len(names)
		}
		chunk := names[i:end]
		var list []Member
		if err := session.Table("member").In("member_name", chunk).Find(&list); err != nil {
			_ = session.Rollback()
			return err
		}
		for j := range list {
			existingByName[list[j].MemberName] = list[j].ID
		}
	}

	// 2. 已有：回填 ID；没有：加入待插入列表
	now := time.Now().Unix()
	var toInsert []*Member
	var newIndices []int
	for i := range members {
		if id, ok := existingByName[members[i].Name]; ok {
			members[i].ID = id // 已有账号，不 insert
			continue
		}
		toInsert = append(toInsert, &Member{
			MemberName: members[i].Name,
			NickName:   members[i].Name,
			Balance:    members[i].Balance,
			Password:   "123456",
			State:      1,
			IsDelete:   0,
			MerchantID: 1,
			Merchant:   "default",
			CreatedAt:  now,
			UpdatedAt:  now,
		})
		newIndices = append(newIndices, i)
	}

	// 3. 批量插入新成员（分批处理避免占位符超限）
	if len(toInsert) > 0 {
		const insertBatchSize = 200 // 插入批次大小，调大以减少插入次数
		for i := 0; i < len(toInsert); i += insertBatchSize {
			end := i + insertBatchSize
			if end > len(toInsert) {
				end = len(toInsert)
			}
			batch := toInsert[i:end]
			if _, err := session.Insert(&batch); err != nil {
				_ = session.Rollback()
				return err
			}
		}

		// 4. 批量查新插入的 ID（IN 分批，避免占位符超限）
		newNames := make([]string, len(toInsert))
		for i := range toInsert {
			newNames[i] = toInsert[i].MemberName
		}
		idByName := make(map[string]int64)
		for i := 0; i < len(newNames); i += inChunkSize {
			end := i + inChunkSize
			if end > len(newNames) {
				end = len(newNames)
			}
			chunk := newNames[i:end]
			var newList []Member
			if err := session.Table("member").In("member_name", chunk).Cols("id", "member_name").Find(&newList); err != nil {
				_ = session.Rollback()
				return err
			}
			for j := range newList {
				idByName[newList[j].MemberName] = newList[j].ID
			}
		}
		for _, idx := range newIndices {
			members[idx].ID = idByName[members[idx].Name]
		}
	}

	return session.Commit()
}
