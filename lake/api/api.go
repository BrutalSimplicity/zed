package api

import (
	"context"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/brimdata/zed"
	"github.com/brimdata/zed/api"
	"github.com/brimdata/zed/api/client"
	"github.com/brimdata/zed/lake"
	"github.com/brimdata/zed/lake/index"
	"github.com/brimdata/zed/lake/pools"
	"github.com/brimdata/zed/lakeparse"
	"github.com/brimdata/zed/order"
	"github.com/brimdata/zed/zbuf"
	"github.com/brimdata/zed/zio"
	"github.com/brimdata/zed/zson"
	"github.com/segmentio/ksuid"
	"go.uber.org/zap"
)

type Interface interface {
	Root() *lake.Root
	Query(ctx context.Context, head *lakeparse.Commitish, src string, srcfiles ...string) (zio.ReadCloser, error)
	QueryWithControl(ctx context.Context, head *lakeparse.Commitish, src string, srcfiles ...string) (zbuf.ProgressReadCloser, error)
	PoolID(ctx context.Context, poolName string) (ksuid.KSUID, error)
	CommitObject(ctx context.Context, poolID ksuid.KSUID, branchName string) (ksuid.KSUID, error)
	CreatePool(context.Context, string, order.Layout, int, int64) (ksuid.KSUID, error)
	RemovePool(context.Context, ksuid.KSUID) error
	RenamePool(context.Context, ksuid.KSUID, string) error
	CreateBranch(ctx context.Context, pool ksuid.KSUID, name string, parent ksuid.KSUID) error
	RemoveBranch(ctx context.Context, pool ksuid.KSUID, branchName string) error
	MergeBranch(ctx context.Context, pool ksuid.KSUID, childBranch, parentBranch string, message api.CommitMessage) (ksuid.KSUID, error)
	Compact(ctx context.Context, pool ksuid.KSUID, branch string, objects []ksuid.KSUID, message api.CommitMessage) (ksuid.KSUID, error)
	Load(ctx context.Context, zctx *zed.Context, pool ksuid.KSUID, branch string, r zio.Reader, message api.CommitMessage) (ksuid.KSUID, error)
	Delete(ctx context.Context, poolID ksuid.KSUID, branchName string, tags []ksuid.KSUID, message api.CommitMessage) (ksuid.KSUID, error)
	DeleteWhere(ctx context.Context, poolID ksuid.KSUID, branchName, src string, commit api.CommitMessage) (ksuid.KSUID, error)
	Revert(ctx context.Context, poolID ksuid.KSUID, branch string, commitID ksuid.KSUID, commit api.CommitMessage) (ksuid.KSUID, error)
	AddIndexRules(context.Context, []index.Rule) error
	DeleteIndexRules(context.Context, []ksuid.KSUID) ([]index.Rule, error)
	ApplyIndexRules(ctx context.Context, rules []string, pool ksuid.KSUID, branchName string, ids []ksuid.KSUID) (ksuid.KSUID, error)
	UpdateIndex(ctx context.Context, names []string, pool ksuid.KSUID, branchName string) (ksuid.KSUID, error)
	AddVectors(ctx context.Context, pool ksuid.KSUID, branch string, objects []ksuid.KSUID, message api.CommitMessage) (ksuid.KSUID, error)
	DeleteVectors(ctx context.Context, pool ksuid.KSUID, branch string, objects []ksuid.KSUID, message api.CommitMessage) (ksuid.KSUID, error)
}

func OpenLake(ctx context.Context, logger *zap.Logger, u string) (Interface, error) {
	if IsLakeService(u) {
		return NewRemoteLake(client.NewConnectionTo(u)), nil
	}
	return OpenLocalLake(ctx, logger, u)
}

func IsLakeService(u string) bool {
	return strings.HasPrefix(u, "http://") || strings.HasPrefix(u, "https://")
}

func ScanIndexRules(ctx context.Context, api Interface) (zio.ReadCloser, error) {
	return api.Query(ctx, nil, "from :index_rules")
}

func GetIndexRules(ctx context.Context, api Interface) ([]index.Rule, error) {
	r, err := ScanIndexRules(ctx, api)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	b := newBuffer(index.FieldRule{}, index.TypeRule{}, index.AggRule{})
	if err := zio.Copy(b, r); err != nil {
		return nil, err
	}
	var rules []index.Rule
	for _, r := range b.results {
		rules = append(rules, r.(index.Rule))
	}
	return rules, nil
}

func LookupPoolByName(ctx context.Context, api Interface, name string) (*pools.Config, error) {
	b := newBuffer(pools.Config{})
	zed := fmt.Sprintf("from :pools | name == '%s'", name)
	q, err := api.Query(ctx, nil, zed)
	if err != nil {
		return nil, err
	}
	defer q.Close()
	if err := zio.Copy(b, zbuf.NoControl(q)); err != nil {
		return nil, err
	}
	switch len(b.results) {
	case 0:
		return nil, fmt.Errorf("%q: pool not found", name)
	case 1:
		pool, ok := b.results[0].(*pools.Config)
		if !ok {
			return nil, fmt.Errorf("internal error: pool record has wrong type: %T", b.results[0])
		}
		return pool, nil
	default:
		return nil, fmt.Errorf("internal error: multiple pools found with same name: %s", name)
	}
}

func GetPools(ctx context.Context, api Interface) ([]*pools.Config, error) {
	b := newBuffer(pools.Config{})
	q, err := api.Query(ctx, nil, "from :pools")
	if err != nil {
		return nil, err
	}
	defer q.Close()
	if err := zio.Copy(b, zbuf.NoControl(q)); err != nil {
		return nil, err
	}
	var pls []*pools.Config
	for _, r := range b.results {
		pls = append(pls, r.(*pools.Config))
	}
	return pls, nil
}

func LookupPoolByID(ctx context.Context, api Interface, id ksuid.KSUID) (*pools.Config, error) {
	b := newBuffer(pools.Config{})
	zed := fmt.Sprintf("from :pools | id == hex('%s')", idToHex(id))
	q, err := api.Query(ctx, nil, zed)
	if err != nil {
		return nil, err
	}
	defer q.Close()
	if err := zio.Copy(b, zbuf.NoControl(q)); err != nil {
		return nil, err
	}
	switch len(b.results) {
	case 0:
		return nil, fmt.Errorf("%s: pool not found", id)
	case 1:
		pool, ok := b.results[0].(*pools.Config)
		if !ok {
			return nil, fmt.Errorf("internal error: pool record has wrong type: %T", b.results[0])
		}
		return pool, nil
	default:
		return nil, fmt.Errorf("internal error: multiple pools found with same id: %s", id)
	}
}

func LookupBranchByName(ctx context.Context, api Interface, poolName, branchName string) (*lake.BranchMeta, error) {
	b := newBuffer(lake.BranchMeta{})
	zed := fmt.Sprintf("from :branches | pool.name == '%s' branch.name == '%s'", poolName, branchName)
	q, err := api.Query(ctx, nil, zed)
	if err != nil {
		return nil, err
	}
	defer q.Close()
	if err := zio.Copy(b, zbuf.NoControl(q)); err != nil {
		return nil, err
	}
	switch len(b.results) {
	case 0:
		return nil, fmt.Errorf("%q: branch not found", poolName+"/"+branchName)
	case 1:
		branch, ok := b.results[0].(*lake.BranchMeta)
		if !ok {
			return nil, fmt.Errorf("internal error: branch record has wrong type: %T", b.results[0])
		}
		return branch, nil
	default:
		return nil, fmt.Errorf("internal error: multiple branches found with same name: %s", poolName+"/"+branchName)
	}
}

func LookupBranchByID(ctx context.Context, api Interface, id ksuid.KSUID) (*lake.BranchMeta, error) {
	b := newBuffer(lake.BranchMeta{})
	zed := fmt.Sprintf("from :branches | branch.id == 'hex(%s)'", idToHex(id))
	q, err := api.Query(ctx, nil, zed)
	if err != nil {
		return nil, err
	}
	defer q.Close()
	if err := zio.Copy(b, zbuf.NoControl(q)); err != nil {
		return nil, err
	}
	switch len(b.results) {
	case 0:
		return nil, fmt.Errorf("%s: branch not found", id)
	case 1:
		branch, ok := b.results[0].(*lake.BranchMeta)
		if !ok {
			return nil, fmt.Errorf("internal error: branch record has wrong type: %T", b.results[0])
		}
		return branch, nil
	default:
		return nil, fmt.Errorf("internal error: multiple branches found with same id: %s", id)
	}
}

func idToHex(id ksuid.KSUID) string {
	return hex.EncodeToString(id.Bytes())
}

type buffer struct {
	unmarshaler *zson.UnmarshalZNGContext
	results     []interface{}
}

var _ zio.Writer = (*buffer)(nil)

func newBuffer(types ...interface{}) *buffer {
	u := zson.NewZNGUnmarshaler()
	u.Bind(types...)
	return &buffer{unmarshaler: u}
}

func (b *buffer) Write(val *zed.Value) error {
	var v interface{}
	if err := b.unmarshaler.Unmarshal(val, &v); err != nil {
		return err
	}
	b.results = append(b.results, v)
	return nil
}
