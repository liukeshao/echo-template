package services

import (
	"context"
	"log/slog"
	"strings"

	"github.com/liukeshao/echo-template/ent"
	"github.com/liukeshao/echo-template/ent/position"
	"github.com/liukeshao/echo-template/ent/user"
	"github.com/liukeshao/echo-template/pkg/errors"
	"github.com/liukeshao/echo-template/pkg/types"
	"github.com/liukeshao/echo-template/pkg/utils"
)

// PositionService 岗位服务
type PositionService struct {
	orm *ent.Client
}

// NewPositionService 创建岗位服务实例
func NewPositionService(orm *ent.Client) *PositionService {
	return &PositionService{
		orm: orm,
	}
}

// toPositionInfo 将岗位实体转换为PositionInfo
func (s *PositionService) toPositionInfo(p *ent.Position) *types.PositionInfo {
	var description *string

	if p.Description != "" {
		description = &p.Description
	}

	return &types.PositionInfo{
		ID:          p.ID,
		Name:        p.Name,
		Code:        p.Code,
		Description: description,
		SortOrder:   p.SortOrder,
		Status:      string(p.Status),
		CreatedAt:   p.CreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt:   p.UpdatedAt.Format("2006-01-02 15:04:05"),
		UserCount:   0, // 将在查询时填充
	}
}

// Create 创建岗位
func (s *PositionService) Create(ctx context.Context, input *types.CreatePositionInput) (*types.PositionOutput, error) {
	// 检查岗位名称是否已存在
	exists, err := s.orm.Position.Query().
		Where(position.NameEQ(input.Name), position.DeletedAtEQ(0)).
		Exist(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "检查岗位名称是否存在失败", "error", err)
		return nil, errors.ErrInternal.Wrapf(err, "检查岗位名称失败")
	}
	if exists {
		return nil, errors.ErrConflict.With("name", input.Name).Errorf("岗位名称已存在")
	}

	// 检查岗位编码是否已存在
	exists, err = s.orm.Position.Query().
		Where(position.CodeEQ(input.Code), position.DeletedAtEQ(0)).
		Exist(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "检查岗位编码是否存在失败", "error", err)
		return nil, errors.ErrInternal.Wrapf(err, "检查岗位编码失败")
	}
	if exists {
		return nil, errors.ErrConflict.With("code", input.Code).Errorf("岗位编码已存在")
	}

	// 设置默认状态
	status := input.Status
	if status == "" {
		status = types.PositionStatusActive
	}

	// 创建岗位
	createQuery := s.orm.Position.Create().
		SetID(utils.GenerateULID()).
		SetName(input.Name).
		SetCode(input.Code).
		SetStatus(position.Status(status)).
		SetSortOrder(input.SortOrder)

	// 设置可选字段
	if input.Description != nil {
		createQuery = createQuery.SetDescription(*input.Description)
	}

	p, err := createQuery.Save(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "创建岗位失败", "error", err)
		return nil, errors.ErrInternal.Wrapf(err, "创建岗位失败")
	}

	// 获取岗位用户数量
	userCount, err := s.orm.User.Query().
		Where(user.PositionIDEQ(p.ID)).
		Count(ctx)
	if err != nil {
		slog.WarnContext(ctx, "获取岗位用户数量失败", "error", err)
	}

	positionInfo := s.toPositionInfo(p)
	positionInfo.UserCount = int64(userCount)

	return &types.PositionOutput{
		PositionInfo: positionInfo,
	}, nil
}

// GetByID 根据ID获取岗位
func (s *PositionService) GetByID(ctx context.Context, positionID string) (*types.PositionOutput, error) {
	p, err := s.orm.Position.Query().
		Where(position.IDEQ(positionID)).
		First(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, errors.ErrNotFound.With("position_id", positionID).Errorf("岗位不存在")
		}
		slog.ErrorContext(ctx, "获取岗位失败", "error", err, "position_id", positionID)
		return nil, errors.ErrInternal.Wrapf(err, "获取岗位失败")
	}

	// 获取岗位用户数量
	userCount, err := s.orm.User.Query().
		Where(user.PositionIDEQ(p.ID)).
		Count(ctx)
	if err != nil {
		slog.WarnContext(ctx, "获取岗位用户数量失败", "error", err)
	}

	positionInfo := s.toPositionInfo(p)
	positionInfo.UserCount = int64(userCount)

	return &types.PositionOutput{
		PositionInfo: positionInfo,
	}, nil
}

// List 获取岗位列表
func (s *PositionService) List(ctx context.Context, input *types.ListPositionsInput) (*types.ListPositionsOutput, error) {
	query := s.orm.Position.Query()

	// 状态筛选
	if input.Status != "" {
		query = query.Where(position.StatusEQ(position.Status(input.Status)))
	}

	// 关键词搜索
	if input.Keyword != "" {
		keyword := strings.TrimSpace(input.Keyword)
		if keyword != "" {
			query = query.Where(
				position.Or(
					position.NameContains(keyword),
					position.CodeContains(keyword),
				),
			)
		}
	}

	// 统计总数
	total, err := query.Count(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "统计岗位总数失败", "error", err)
		return nil, errors.ErrInternal.Wrapf(err, "统计岗位总数失败")
	}

	// 分页和排序
	query = query.
		Order(ent.Asc(position.FieldSortOrder), ent.Asc(position.FieldCreatedAt)).
		Limit(input.PageSize).
		Offset((input.Page - 1) * input.PageSize)

	positions, err := query.All(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "获取岗位列表失败", "error", err)
		return nil, errors.ErrInternal.Wrapf(err, "获取岗位列表失败")
	}

	// 转换为输出格式
	positionInfos := make([]*types.PositionInfo, 0, len(positions))
	for _, p := range positions {
		// 获取每个岗位的用户数量
		userCount, err := s.orm.User.Query().
			Where(user.PositionIDEQ(p.ID)).
			Count(ctx)
		if err != nil {
			slog.WarnContext(ctx, "获取岗位用户数量失败", "error", err, "position_id", p.ID)
			userCount = 0
		}

		positionInfo := s.toPositionInfo(p)
		positionInfo.UserCount = int64(userCount)
		positionInfos = append(positionInfos, positionInfo)
	}

	return &types.ListPositionsOutput{
		Positions:  positionInfos,
		PageOutput: types.NewPageOutput(input.PageInput, total),
	}, nil
}

// Update 更新岗位
func (s *PositionService) Update(ctx context.Context, positionID string, input *types.UpdatePositionInput) (*types.PositionOutput, error) {
	// 检查岗位是否存在
	p, err := s.orm.Position.Query().
		Where(position.IDEQ(positionID)).
		First(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, errors.ErrNotFound.With("position_id", positionID).Errorf("岗位不存在")
		}
		slog.ErrorContext(ctx, "获取岗位失败", "error", err, "position_id", positionID)
		return nil, errors.ErrInternal.Wrapf(err, "获取岗位失败")
	}

	// 检查岗位名称是否已被其他岗位使用
	if input.Name != nil && *input.Name != p.Name {
		exists, err := s.orm.Position.Query().
			Where(
				position.NameEQ(*input.Name),
				position.IDNEQ(positionID),
				position.DeletedAtEQ(0),
			).
			Exist(ctx)
		if err != nil {
			slog.ErrorContext(ctx, "检查岗位名称是否存在失败", "error", err)
			return nil, errors.ErrInternal.Wrapf(err, "检查岗位名称失败")
		}
		if exists {
			return nil, errors.ErrConflict.With("name", *input.Name).Errorf("岗位名称已存在")
		}
	}

	// 检查岗位编码是否已被其他岗位使用
	if input.Code != nil && *input.Code != p.Code {
		exists, err := s.orm.Position.Query().
			Where(
				position.CodeEQ(*input.Code),
				position.IDNEQ(positionID),
				position.DeletedAtEQ(0),
			).
			Exist(ctx)
		if err != nil {
			slog.ErrorContext(ctx, "检查岗位编码是否存在失败", "error", err)
			return nil, errors.ErrInternal.Wrapf(err, "检查岗位编码失败")
		}
		if exists {
			return nil, errors.ErrConflict.With("code", *input.Code).Errorf("岗位编码已存在")
		}
	}

	// 更新岗位
	updateQuery := s.orm.Position.UpdateOneID(positionID)

	if input.Name != nil {
		updateQuery = updateQuery.SetName(*input.Name)
	}
	if input.Code != nil {
		updateQuery = updateQuery.SetCode(*input.Code)
	}
	if input.Description != nil {
		updateQuery = updateQuery.SetDescription(*input.Description)
	}
	if input.SortOrder != nil {
		updateQuery = updateQuery.SetSortOrder(*input.SortOrder)
	}
	if input.Status != nil {
		updateQuery = updateQuery.SetStatus(position.Status(*input.Status))
	}

	updatedPosition, err := updateQuery.Save(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "更新岗位失败", "error", err, "position_id", positionID)
		return nil, errors.ErrInternal.Wrapf(err, "更新岗位失败")
	}

	// 获取岗位用户数量
	userCount, err := s.orm.User.Query().
		Where(user.PositionIDEQ(updatedPosition.ID)).
		Count(ctx)
	if err != nil {
		slog.WarnContext(ctx, "获取岗位用户数量失败", "error", err)
	}

	positionInfo := s.toPositionInfo(updatedPosition)
	positionInfo.UserCount = int64(userCount)

	return &types.PositionOutput{
		PositionInfo: positionInfo,
	}, nil
}

// Sort 批量更新岗位排序
func (s *PositionService) Sort(ctx context.Context, input *types.SortPositionInput) error {
	// 验证所有岗位是否存在
	positionIDs := make([]string, 0, len(input.PositionSorts))
	for _, item := range input.PositionSorts {
		positionIDs = append(positionIDs, item.ID)
	}

	count, err := s.orm.Position.Query().
		Where(position.IDIn(positionIDs...)).
		Count(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "验证岗位存在性失败", "error", err)
		return errors.ErrInternal.Wrapf(err, "验证岗位存在性失败")
	}
	if count != len(positionIDs) {
		return errors.ErrBadRequest.Errorf("存在无效的岗位ID")
	}

	// 开启事务
	tx, err := s.orm.Tx(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "开启事务失败", "error", err)
		return errors.ErrInternal.Wrapf(err, "开启事务失败")
	}
	defer tx.Rollback()

	// 批量更新排序
	for _, item := range input.PositionSorts {
		err = tx.Position.UpdateOneID(item.ID).
			SetSortOrder(item.SortOrder).
			Exec(ctx)
		if err != nil {
			slog.ErrorContext(ctx, "更新岗位排序失败", "error", err, "position_id", item.ID)
			return errors.ErrInternal.Wrapf(err, "更新岗位排序失败")
		}
	}

	// 提交事务
	if err = tx.Commit(); err != nil {
		slog.ErrorContext(ctx, "提交事务失败", "error", err)
		return errors.ErrInternal.Wrapf(err, "提交事务失败")
	}

	return nil
}

// CheckDeletable 检查岗位是否可以删除
func (s *PositionService) CheckDeletable(ctx context.Context, positionID string) (*types.CheckPositionDeletableOutput, error) {
	// 检查岗位是否存在
	exists, err := s.orm.Position.Query().
		Where(position.IDEQ(positionID)).
		Exist(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "检查岗位是否存在失败", "error", err, "position_id", positionID)
		return nil, errors.ErrInternal.Wrapf(err, "检查岗位是否存在失败")
	}
	if !exists {
		return nil, errors.ErrNotFound.With("position_id", positionID).Errorf("岗位不存在")
	}

	// 检查关联用户数量
	userCount, err := s.orm.User.Query().
		Where(user.PositionIDEQ(positionID)).
		Count(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "获取岗位关联用户数量失败", "error", err, "position_id", positionID)
		return nil, errors.ErrInternal.Wrapf(err, "获取岗位关联用户数量失败")
	}

	output := &types.CheckPositionDeletableOutput{
		UserCount: int64(userCount),
	}

	if userCount > 0 {
		output.Deletable = false
		output.Reason = "岗位下还有关联用户，无法删除"
	} else {
		output.Deletable = true
		output.Reason = ""
	}

	return output, nil
}

// Delete 删除岗位（逻辑删除）
func (s *PositionService) Delete(ctx context.Context, positionID string) error {
	// 先检查是否可以删除
	checkResult, err := s.CheckDeletable(ctx, positionID)
	if err != nil {
		return err
	}

	if !checkResult.Deletable {
		return errors.ErrBadRequest.Errorf("%s", checkResult.Reason)
	}

	// 执行删除
	err = s.orm.Position.DeleteOneID(positionID).Exec(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return errors.ErrNotFound.With("position_id", positionID).Errorf("岗位不存在")
		}
		slog.ErrorContext(ctx, "删除岗位失败", "error", err, "position_id", positionID)
		return errors.ErrInternal.Wrapf(err, "删除岗位失败")
	}

	return nil
}

// Stats 获取岗位统计信息
func (s *PositionService) Stats(ctx context.Context) (*types.PositionStatsOutput, error) {
	// 统计各状态岗位数量
	totalPositions, err := s.orm.Position.Query().
		Count(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "统计总岗位数失败", "error", err)
		return nil, errors.ErrInternal.Wrapf(err, "统计总岗位数失败")
	}

	activePositions, err := s.orm.Position.Query().
		Where(position.StatusEQ(position.StatusActive)).
		Count(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "统计启用岗位数失败", "error", err)
		return nil, errors.ErrInternal.Wrapf(err, "统计启用岗位数失败")
	}

	inactivePositions, err := s.orm.Position.Query().
		Where(position.StatusEQ(position.StatusInactive)).
		Count(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "统计停用岗位数失败", "error", err)
		return nil, errors.ErrInternal.Wrapf(err, "统计停用岗位数失败")
	}

	return &types.PositionStatsOutput{
		TotalPositions:    int64(totalPositions),
		ActivePositions:   int64(activePositions),
		InactivePositions: int64(inactivePositions),
		StatusBreakdown: map[string]int64{
			types.PositionStatusActive:   int64(activePositions),
			types.PositionStatusInactive: int64(inactivePositions),
		},
	}, nil
}
