package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"

	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/model"
	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/transaction_settings/dto"
	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/transaction_settings/service"
	"gitea.tagsamurai.local/BETS-V2/ts-utils/v2/enum"
	tsMiddleware "gitea.tagsamurai.local/BETS-V2/ts-utils/v2/middleware"
)

// fakeOverrideRepo is a settings repository with in-memory override answers.
type fakeOverrideRepo struct {
	*fakeRepo
	records []dto.OverrideListRecord
	byID    []model.TransactionRuleOverride
	dups    []int64
	fail    error
	otx     *fakeOvTx
	opts    service.OverrideOptionValues
}

type fakeOvTx struct{ inserted, deleted int }

func (t *fakeOvTx) InsertOverrides(_ context.Context, rows []model.TransactionRuleOverride) error {
	t.inserted += len(rows)
	return nil
}
func (t *fakeOvTx) DeleteOverrides(_ context.Context, ids []int64) (int64, error) {
	t.deleted += len(ids)
	return int64(len(ids)), nil
}
func (t *fakeOvTx) BumpSettingsVersion(context.Context, int64, string) error { return nil }
func (t *fakeOvTx) Commit(context.Context) error                             { return nil }
func (t *fakeOvTx) Rollback(context.Context) error                           { return nil }
func (t *fakeOvTx) Close()                                                   {}

func (f *fakeOverrideRepo) ListOverrides(context.Context, string, dto.OverrideQuery) ([]dto.OverrideListRecord, int, error) {
	return f.records, len(f.records), f.fail
}
func (f *fakeOverrideRepo) FindOverrideOptions(context.Context, string, dto.OverrideQuery) (service.OverrideOptionValues, error) {
	return f.opts, f.fail
}
func (f *fakeOverrideRepo) FindOverridesByIDs(_ context.Context, _ string, ids []int64) ([]model.TransactionRuleOverride, error) {
	var out []model.TransactionRuleOverride
	for _, o := range f.byID {
		for _, id := range ids {
			if o.ID == id {
				out = append(out, o)
			}
		}
	}
	return out, f.fail
}
func (f *fakeOverrideRepo) FindOverridesForTargets(context.Context, string, string, []int64) ([]model.TransactionRuleOverride, error) {
	return f.byID, f.fail
}
func (f *fakeOverrideRepo) FindDuplicateTargetIDs(context.Context, string, string, string, *int64, []int64) ([]int64, error) {
	return f.dups, f.fail
}
func (f *fakeOverrideRepo) BeginOverrideWrite(context.Context, string) (service.OverrideTx, error) {
	if f.fail != nil {
		return nil, f.fail
	}
	if f.otx == nil {
		f.otx = &fakeOvTx{}
	}
	return f.otx, nil
}

type fakeTargets struct{ list []service.OverrideTarget }

func (f fakeTargets) ResolveTargets(_ context.Context, _, _ string, ids []int64) ([]service.OverrideTarget, error) {
	var out []service.OverrideTarget
	for _, t := range f.list {
		for _, id := range ids {
			if t.ID == id {
				out = append(out, t)
			}
		}
	}
	return out, nil
}

func totalControlIdentity(fiber.Ctx) (Identity, error) {
	return Identity{ClientID: "c1", UserID: 7, FullName: "Rina Wulandari", Access: service.CallerAccess{TotalControl: true, ViewAll: true, AssetNameWrite: true}}, nil
}

func overrideApp(repo *fakeOverrideRepo, id IdentityReader, configure ...func(*service.Service)) *fiber.App {
	svc := service.NewService(repo, nil)
	for _, c := range configure {
		c(svc)
	}
	h := NewHandler(svc, fullPlan).WithIdentity(id)
	app := fiber.New()
	app.Post("/overrides", h.CreateOverrides)
	app.Post("/overrides/list", h.ListOverrides)
	app.Post("/overrides/options", h.OverridesOptions)
	app.Delete("/overrides", h.DeleteOverrides)
	return app
}

func newOverrideRepo() *fakeOverrideRepo { return &fakeOverrideRepo{fakeRepo: &fakeRepo{}} }

func withTargets(ts ...service.OverrideTarget) func(*service.Service) {
	return func(s *service.Service) { s.WithOverrideTargetDirectory(fakeTargets{list: ts}) }
}

const createBody = `{"scope":"group","targetIds":["10"],"transaction":"borrow","fields":[{"key":"max_duration","value":30}]}`

func TestCreateOverrides_Created(t *testing.T) {
	repo := newOverrideRepo()
	app := overrideApp(repo, totalControlIdentity, withTargets(service.OverrideTarget{ID: 10, Exists: true, Name: "G", GroupChain: []int64{10}}))
	code, body := send(t, app, http.MethodPost, "/overrides", createBody)
	require.Equal(t, http.StatusCreated, code)
	require.Contains(t, body, "Success, override has been added.")
	require.Equal(t, 1, repo.otx.inserted)
}

func TestCreateOverrides_NotFound(t *testing.T) {
	// An unknown Group target is 404 and nothing is created.
	code, body := send(t, overrideApp(newOverrideRepo(), totalControlIdentity, withTargets()), http.MethodPost, "/overrides", createBody)
	require.Equal(t, http.StatusNotFound, code)
	require.Contains(t, body, "Not found")
	// An unknown or deleted Category is 404 as well.
	categoryBody := `{"scope":"group","targetIds":["10"],"transaction":"borrow","categoryId":"5","fields":[{"key":"max_duration","value":30}]}`
	withCategory := func(s *service.Service) { s.WithCategoryDirectory(fakeCategories{}) }
	code, body = send(t, overrideApp(newOverrideRepo(), totalControlIdentity, withTargets(service.OverrideTarget{ID: 10, Exists: true, Name: "G", GroupChain: []int64{10}}), withCategory), http.MethodPost, "/overrides", categoryBody)
	require.Equal(t, http.StatusNotFound, code)
	require.Contains(t, body, "Not found")
}

func TestCreateOverrides_Failures(t *testing.T) {
	target := withTargets(service.OverrideTarget{ID: 10, Exists: true, Name: "G", GroupChain: []int64{10}})
	app := overrideApp(newOverrideRepo(), totalControlIdentity, target)
	code, body := send(t, app, http.MethodPost, "/overrides", `{"scope":""}`)
	require.Equal(t, http.StatusUnprocessableEntity, code)
	require.Contains(t, body, "You must pick a scope")
	code, body = send(t, app, http.MethodPost, "/overrides", `{"scope": 5}`)
	require.Equal(t, http.StatusBadRequest, code)
	require.Contains(t, body, "Invalid request body")

	code, _ = send(t, overrideApp(newOverrideRepo(), func(fiber.Ctx) (Identity, error) { return Identity{}, io.EOF }), http.MethodPost, "/overrides", createBody)
	require.Equal(t, http.StatusUnauthorized, code)

	readOnly := func(fiber.Ctx) (Identity, error) {
		return Identity{ClientID: "c1", UserID: 7, Access: service.CallerAccess{ReadOnly: true}}, nil
	}
	code, body = send(t, overrideApp(newOverrideRepo(), readOnly, target), http.MethodPost, "/overrides", createBody)
	require.Equal(t, http.StatusForbidden, code)
	require.Contains(t, body, "Forbidden")

	dup := newOverrideRepo()
	dup.dups = []int64{10}
	code, body = send(t, overrideApp(dup, totalControlIdentity, target), http.MethodPost, "/overrides", createBody)
	require.Equal(t, http.StatusConflict, code)
	var parsed struct {
		Message string                    `json:"message"`
		Data    dto.DuplicateOverrideData `json:"data"`
	}
	require.NoError(t, json.Unmarshal([]byte(body), &parsed))
	require.Equal(t, []string{"10"}, parsed.Data.DuplicateTargetIDs)
	require.Contains(t, parsed.Message, "An active override already exists")

	broken := newOverrideRepo()
	broken.fail = errors.New("db down")
	code, body = send(t, overrideApp(broken, totalControlIdentity, target), http.MethodPost, "/overrides", createBody)
	require.Equal(t, http.StatusInternalServerError, code)
	require.Contains(t, body, "Error, failed to add override.")

	// No override storage wired: the same 500.
	code, _ = send(t, overrideApp(newOverrideRepo(), totalControlIdentity), http.MethodPost, "/overrides", createBody)
	require.Equal(t, http.StatusInternalServerError, code)
}

func TestOverrideHandlers_PlanFailure(t *testing.T) {
	h := NewHandler(service.NewService(newOverrideRepo(), nil).WithOverrideTargetDirectory(fakeTargets{list: []service.OverrideTarget{{ID: 10, Exists: true, GroupChain: []int64{10}}}}), failingPlan).WithIdentity(totalControlIdentity)
	app := fiber.New()
	app.Post("/overrides", h.CreateOverrides)
	app.Post("/overrides/list", h.ListOverrides)
	app.Delete("/overrides", h.DeleteOverrides)

	code, body := send(t, app, http.MethodPost, "/overrides", createBody)
	require.Equal(t, http.StatusInternalServerError, code)
	require.Contains(t, body, "Error, failed to add override.")
	code, body = send(t, app, http.MethodPost, "/overrides/list", `{"columns":["scope"],"transaction":"borrow"}`)
	require.Equal(t, http.StatusInternalServerError, code)
	require.Contains(t, body, "Internal server error")
	code, body = send(t, app, http.MethodDelete, "/overrides", `{"ids":["1"]}`)
	require.Equal(t, http.StatusInternalServerError, code)
	require.Contains(t, body, "Error, failed to delete override.")
}

func TestListAndOptionsHandlers(t *testing.T) {
	repo := newOverrideRepo()
	repo.records = []dto.OverrideListRecord{{ID: "1", Scope: "group", TargetID: "10", Transaction: "borrow", Fields: json.RawMessage(`{"max_duration":3}`), CreatedBy: "Rina"}}
	repo.opts = service.OverrideOptionValues{CreatedBy: []string{"Rina"}}
	app := overrideApp(repo, totalControlIdentity, withTargets(service.OverrideTarget{ID: 10, Exists: true}))
	code, body := send(t, app, http.MethodPost, "/overrides/list", `{"columns":["scope"],"transaction":"borrow"}`)
	require.Equal(t, http.StatusOK, code)
	var list struct {
		Data dto.ListOverridesData `json:"data"`
	}
	require.NoError(t, json.Unmarshal([]byte(body), &list))
	require.Equal(t, 1, list.Data.TotalRecords)
	require.Equal(t, "Max duration", list.Data.Data[0].Overridden[0].Label)

	code, body = send(t, app, http.MethodPost, "/overrides/list", `{"transaction":"borrow"}`)
	require.Equal(t, http.StatusUnprocessableEntity, code)
	require.Contains(t, body, "columns")
	code, _ = send(t, app, http.MethodPost, "/overrides/list", `{`)
	require.Equal(t, http.StatusBadRequest, code)
	// Providers missing for a non-empty page: 500, not a stub.
	code, _ = send(t, overrideApp(repo, totalControlIdentity), http.MethodPost, "/overrides/list", `{"columns":["scope"],"transaction":"borrow"}`)
	require.Equal(t, http.StatusInternalServerError, code)

	code, body = send(t, app, http.MethodPost, "/overrides/options", `{"transaction":"borrow","createdBy":true}`)
	require.Equal(t, http.StatusOK, code)
	require.Contains(t, body, `"createdBy":[{"label":"Rina","value":"Rina"}]`)
	require.NotContains(t, body, `"scope"`)
	code, _ = send(t, app, http.MethodPost, "/overrides/options", `{"transaction":"loan"}`)
	require.Equal(t, http.StatusUnprocessableEntity, code)
}

func TestDeleteOverridesHandler(t *testing.T) {
	repo := newOverrideRepo()
	repo.byID = []model.TransactionRuleOverride{{ID: 1, Scope: "group", TransactionType: "borrow", TargetID: 10, Fields: json.RawMessage(`{}`)},
		{ID: 2, Scope: "asset_name", TransactionType: "assign", TargetID: 20, Fields: json.RawMessage(`{}`)}}
	app := overrideApp(repo, totalControlIdentity)
	code, body := send(t, app, http.MethodDelete, "/overrides", `{"ids":["1","9"]}`)
	require.Equal(t, http.StatusOK, code)
	require.Contains(t, body, "Success, override has been deleted.")
	require.Contains(t, body, `"deletedIds":["1","9"]`)
	require.Contains(t, body, `"skipped":[]`)

	code, body = send(t, app, http.MethodDelete, "/overrides", `{"ids":["7","8"]}`)
	require.Equal(t, http.StatusNoContent, code)
	require.Empty(t, body)

	code, _ = send(t, app, http.MethodDelete, "/overrides", `{"ids":[]}`)
	require.Equal(t, http.StatusUnprocessableEntity, code)

	partial := func(fiber.Ctx) (Identity, error) {
		return Identity{ClientID: "c1", UserID: 7, Access: service.CallerAccess{AssetNameWrite: true}}, nil
	}
	code, body = send(t, overrideApp(repo, partial), http.MethodDelete, "/overrides", `{"ids":["1"]}`)
	require.Equal(t, http.StatusForbidden, code)
	require.Contains(t, body, "Forbidden")

	// Partial: the Asset Name row goes, the Group row is out of scope.
	repo.byID[1].TransactionType = "borrow"
	code, body = send(t, overrideApp(repo, partial), http.MethodDelete, "/overrides", `{"ids":["1","2"]}`)
	require.Equal(t, http.StatusOK, code)
	require.Contains(t, body, "Some items were skipped.")
	require.Contains(t, body, `"reason":"out_of_scope"`)

	broken := newOverrideRepo()
	broken.fail = errors.New("db down")
	code, body = send(t, overrideApp(broken, totalControlIdentity), http.MethodDelete, "/overrides", `{"ids":["1"]}`)
	require.Equal(t, http.StatusInternalServerError, code)
	require.Contains(t, body, "Error, failed to delete override.")
}

func TestDeleteOverridesHandler_GuardConflict(t *testing.T) {
	repo := newOverrideRepo()
	one := model.TransactionRuleOverride{ID: 1, Scope: "group", TransactionType: "assign", TargetID: 10, Fields: json.RawMessage(`{"allowed_assignee_employee":true}`)}
	two := model.TransactionRuleOverride{ID: 2, Scope: "group", TransactionType: "assign", TargetID: 10, Fields: json.RawMessage(`{"allowed_assignee_employee":false,"allowed_assignee_division":false}`)}
	repo.byID = []model.TransactionRuleOverride{one, two}
	code, body := send(t, overrideApp(repo, totalControlIdentity), http.MethodDelete, "/overrides", `{"ids":["1"]}`)
	require.Equal(t, http.StatusConflict, code)
	require.Contains(t, body, "At least one assignee type must be enabled")
	require.Contains(t, body, `"reason":"assignee_guard"`)
}

func TestCallerAccess(t *testing.T) {
	enumAsset := enum.ModuleAccessFixedAsset
	tc := callerAccess(tsMiddleware.UserAccessCache{ModuleAccess: map[enum.ModuleAccess]tsMiddleware.ModuleFlags{enumAsset: {IsTotalControl: true}}})
	require.True(t, tc.TotalControl && tc.ViewAll && tc.AssetNameWrite)
	ro := callerAccess(tsMiddleware.UserAccessCache{ModuleAccess: map[enum.ModuleAccess]tsMiddleware.ModuleFlags{enumAsset: {IsReadOnly: true}}})
	require.True(t, ro.ReadOnly && ro.ViewAll)
	require.False(t, ro.AssetNameWrite)

	var roles tsMiddleware.UserAccessCache
	roles.FixedAssetSystemRole.GlobalRole = map[enum.SystemRoleKey]tsMiddleware.Permissions[bool]{
		enum.SystemRoleKeyManageTransactionSettings: {Edit: true, View: true},
	}
	roles.FixedAssetSystemRole.GroupRoles = map[enum.SystemRoleKey]tsMiddleware.Permissions[[]string]{
		enum.SystemRoleKeyManageAssetAndTAG: {View: []string{"4"}, Edit: []string{"3"}},
	}
	got := callerAccess(roles)
	require.True(t, got.AssetNameWrite && got.ViewAll)
	require.False(t, got.TotalControl)
	require.Equal(t, []int64{3}, got.WriteGroupIDs)
	require.ElementsMatch(t, []int64{3, 4}, got.ViewGroupIDs)
}

func TestOverrideHandlers_AuthAndBind(t *testing.T) {
	denied := func(fiber.Ctx) (Identity, error) { return Identity{}, io.EOF }
	for _, tc := range []struct{ method, path string }{
		{http.MethodPost, "/overrides/options"}, {http.MethodPost, "/overrides/list"}, {http.MethodDelete, "/overrides"},
	} {
		code, _ := send(t, overrideApp(newOverrideRepo(), denied), tc.method, tc.path, `{}`)
		require.Equal(t, http.StatusUnauthorized, code, tc.path)
		code, _ = send(t, overrideApp(newOverrideRepo(), totalControlIdentity), tc.method, tc.path, `{"transaction": 5, "ids": 5}`)
		require.Equal(t, http.StatusBadRequest, code, tc.path)
	}
}

func TestSaveValidationFailed_NonViolationError(t *testing.T) {
	app := fiber.New()
	app.Post("/x", func(c fiber.Ctx) error { return saveValidationFailed(c, dto.SaveSettingsRequest{}, io.EOF) })
	code, _ := send(t, app, http.MethodPost, "/x", "")
	require.Equal(t, http.StatusUnprocessableEntity, code)
}
