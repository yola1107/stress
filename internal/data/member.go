package data

import (
	"context"
	"time"

	"stress/internal/biz/member"

	"xorm.io/xorm"
)

const (
	inChunkSize       = 250 // MySQL IN 子句分批大小
	insertBatchSize   = 200 // 插入批次大小
	defaultMerchant   = "default"
	defaultPassword   = "123456"
	defaultMerchantID = 1
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

// BatchUpsertMembers 批量插入或更新成员（已有则回填ID，没有则插入）
func (r *dataRepo) BatchUpsertMembers(ctx context.Context, members []member.Info) error {
	if len(members) == 0 {
		return nil
	}

	session := r.data.db.NewSession().Context(ctx)
	defer session.Close()

	if err := session.Begin(); err != nil {
		return err
	}

	// 1. 查询已存在的成员
	existingByName := r.queryExistingMembers(session, members)

	// 2. 分离已存在和待插入的成员
	toInsert, newIndices := r.separateMembers(members, existingByName)

	// 3. 批量插入新成员并回填 ID
	if len(toInsert) > 0 {
		if err := r.batchInsertMembers(session, toInsert, members, newIndices); err != nil {
			_ = session.Rollback()
			return err
		}
	}

	return session.Commit()
}

// queryExistingMembers 查询已存在的成员
func (r *dataRepo) queryExistingMembers(session *xorm.Session, members []member.Info) map[string]int64 {
	names := make([]string, len(members))
	for i := range members {
		names[i] = members[i].Name
	}

	existingByName := make(map[string]int64)
	for i := 0; i < len(names); i += inChunkSize {
		end := min(i+inChunkSize, len(names))
		var list []Member
		if err := session.Table("member").In("member_name", names[i:end]).Find(&list); err != nil {
			r.log.Warnf("query existing members chunk [%d:%d] failed: %v", i, end, err)
			continue
		}
		for j := range list {
			existingByName[list[j].MemberName] = list[j].ID
		}
	}
	return existingByName
}

// separateMembers 分离已存在和待插入的成员
func (r *dataRepo) separateMembers(members []member.Info, existingByName map[string]int64) ([]*Member, []int) {
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
			Password:   defaultPassword,
			State:      1,
			IsDelete:   0,
			MerchantID: defaultMerchantID,
			Merchant:   defaultMerchant,
			CreatedAt:  now,
			UpdatedAt:  now,
		})
		newIndices = append(newIndices, i)
	}
	return toInsert, newIndices
}

// batchInsertMembers 批量插入成员
func (r *dataRepo) batchInsertMembers(session *xorm.Session, toInsert []*Member, members []member.Info, newIndices []int) error {
	// 批量插入
	for i := 0; i < len(toInsert); i += insertBatchSize {
		end := min(i+insertBatchSize, len(toInsert))
		if _, err := session.Insert(toInsert[i:end]); err != nil {
			return err
		}
	}

	// 查询新插入的 ID
	names := make([]string, len(toInsert))
	for i := range toInsert {
		names[i] = toInsert[i].MemberName
	}

	idByName := make(map[string]int64)
	for i := 0; i < len(names); i += inChunkSize {
		end := min(i+inChunkSize, len(names))
		var list []Member
		if err := session.Table("member").In("member_name", names[i:end]).Cols("id", "member_name").Find(&list); err != nil {
			return err
		}
		for j := range list {
			idByName[list[j].MemberName] = list[j].ID
		}
	}

	// 回填 ID
	for _, idx := range newIndices {
		members[idx].ID = idByName[members[idx].Name]
	}
	return nil
}
