package router

import (
	"net/http"
	"slices"
	"strings"
	"testing"
)

type productGroup struct {
	Id         string   `json:"id"`
	Name       string   `json:"name"`
	Slug       string   `json:"slug"`
	Position   int      `json:"position"`
	ProductIds []string `json:"product_ids"`
}

func (s shop) createGroup(name string) productGroup {
	s.t.Helper()
	code, env := s.do(http.MethodPost, "/store/"+s.storeId+"/groups", s.owner, map[string]any{"name": name})
	if code != http.StatusCreated {
		s.t.Fatalf("create group %q: %d %s", name, code, env.Message)
	}
	var g productGroup
	s.data(env, &g)
	return g
}

func (s shop) addToGroup(group string, products ...string) (int, productGroup) {
	s.t.Helper()
	code, env := s.do(http.MethodPost, "/store/"+s.storeId+"/groups/"+group+"/products", s.owner, map[string]any{"product_ids": products})
	var g productGroup
	if code == http.StatusOK {
		s.data(env, &g)
	}
	return code, g
}

func (s shop) listGroups() []productGroup {
	s.t.Helper()
	code, env := s.do(http.MethodGet, "/store/"+s.storeId+"/groups", s.owner, nil)
	if code != http.StatusOK {
		s.t.Fatalf("list groups: %d %s", code, env.Message)
	}
	var g []productGroup
	s.data(env, &g)
	return g
}

func (s shop) storefrontProductIds(path string) (int, []string) {
	s.t.Helper()
	w := s.get(path, s.key)
	var list struct {
		Items []struct {
			Id string `json:"id"`
		} `json:"items"`
	}
	if w.Code != http.StatusOK {
		return w.Code, nil
	}
	decode(s.t, w, &list)
	ids := []string{}
	for _, p := range list.Items {
		ids = append(ids, p.Id)
	}
	return w.Code, ids
}

func TestProductGroups_CreateAddListMove(t *testing.T) {
	s := newShop(t)

	if g := s.listGroups(); len(g) != 0 {
		t.Fatalf("new store has groups: %+v", g)
	}
	bags := s.createGroup("  Ankara   Bags ")
	if bags.Name != "Ankara Bags" || bags.Slug != "ankara-bags" || bags.Position != 0 || len(bags.ProductIds) != 0 {
		t.Fatalf("created: %+v", bags)
	}
	caps := s.createGroup("Ìlá Caps")
	if caps.Slug != "ila-caps" || caps.Position != 1 {
		t.Errorf("accents and position: %+v", caps)
	}

	if code, g := s.addToGroup(bags.Id, s.bag, s.cap, s.bag); code != http.StatusOK || len(g.ProductIds) != 2 {
		t.Fatalf("add: %d %+v", code, g)
	}
	// A product is in one group only: adding it elsewhere moves it.
	if code, g := s.addToGroup(caps.Id, s.cap); code != http.StatusOK || len(g.ProductIds) != 1 || g.ProductIds[0] != s.cap {
		t.Fatalf("move: %d %+v", code, g)
	}
	groups := s.listGroups()
	if len(groups) != 2 || groups[0].Id != bags.Id || len(groups[0].ProductIds) != 1 || groups[0].ProductIds[0] != s.bag {
		t.Errorf("after move: %+v", groups)
	}

	// The seller reorders: caps first.
	if code, env := s.do(http.MethodPut, "/store/"+s.storeId+"/groups/"+caps.Id, s.owner, map[string]any{"position": -1}); code != http.StatusOK {
		t.Fatalf("reorder: %d %s", code, env.Message)
	}
	if g := s.listGroups(); g[0].Id != caps.Id {
		t.Errorf("order after reorder: %+v", g)
	}

	if code, env := s.do(http.MethodDelete, "/store/"+s.storeId+"/groups/"+caps.Id+"/products/"+s.cap, s.owner, nil); code != http.StatusOK {
		t.Fatalf("remove product: %d %s", code, env.Message)
	}
	if code, _ := s.do(http.MethodDelete, "/store/"+s.storeId+"/groups/"+caps.Id+"/products/"+s.cap, s.owner, nil); code != http.StatusNotFound {
		t.Errorf("remove again: %d, want 404", code)
	}
}

func TestProductGroups_RenameAndDelete(t *testing.T) {
	s := newShop(t)
	bags := s.createGroup("Bags")
	s.createGroup("Shoes")
	s.addToGroup(bags.Id, s.bag)

	code, env := s.do(http.MethodPut, "/store/"+s.storeId+"/groups/"+bags.Id, s.owner, map[string]any{"name": "Shoes"})
	if code != http.StatusConflict {
		t.Errorf("rename to a taken name: %d %s, want 409", code, env.Message)
	}
	code, env = s.do(http.MethodPut, "/store/"+s.storeId+"/groups/"+bags.Id, s.owner, map[string]any{"name": "Leather Bags"})
	var renamed productGroup
	s.data(env, &renamed)
	if code != http.StatusOK || renamed.Slug != "leather-bags" || len(renamed.ProductIds) != 1 {
		t.Errorf("rename: %d %+v", code, renamed)
	}

	if code, _ := s.do(http.MethodDelete, "/store/"+s.storeId+"/groups/"+bags.Id, s.owner, nil); code != http.StatusOK {
		t.Fatalf("delete: %d", code)
	}
	if code, _ := s.do(http.MethodDelete, "/store/"+s.storeId+"/groups/"+bags.Id, s.owner, nil); code != http.StatusNotFound {
		t.Errorf("delete again: %d, want 404", code)
	}
	// The product survives, ungrouped.
	if code, ids := s.storefrontProductIds("/storefront/products?ungrouped=true"); code != http.StatusOK || !slices.Contains(ids, s.bag) {
		t.Errorf("product after its group was deleted: %d %v", code, ids)
	}
}

func TestProductGroups_Rejected(t *testing.T) {
	s := newShop(t)
	bags := s.createGroup("Bags")

	for _, name := range []string{"", "   ", "This group name is far too long to show on a shop page"} {
		if code, _ := s.do(http.MethodPost, "/store/"+s.storeId+"/groups", s.owner, map[string]any{"name": name}); code != http.StatusBadRequest {
			t.Errorf("name %q: %d, want 400", name, code)
		}
	}
	if code, _ := s.do(http.MethodPost, "/store/"+s.storeId+"/groups", s.owner, map[string]any{"name": "bags"}); code != http.StatusConflict {
		t.Errorf("same name, other case: %d, want 409", code)
	}
	// Another store's product can't be added, and nothing moves.
	if code, _ := s.addToGroup(bags.Id, s.bag, s.foreign); code != http.StatusNotFound {
		t.Errorf("foreign product: %d, want 404", code)
	}
	if g := s.listGroups(); len(g[0].ProductIds) != 0 {
		t.Errorf("a product moved in a refused request: %+v", g)
	}
	if code, _ := s.addToGroup(bags.Id); code != http.StatusBadRequest {
		t.Errorf("no products: %d, want 400", code)
	}

	// Only the owner manages groups.
	for _, req := range []struct{ method, path string }{
		{http.MethodGet, "/store/" + s.storeId + "/groups"},
		{http.MethodPost, "/store/" + s.storeId + "/groups"},
		{http.MethodPut, "/store/" + s.storeId + "/groups/" + bags.Id},
		{http.MethodDelete, "/store/" + s.storeId + "/groups/" + bags.Id},
	} {
		if code, _ := s.do(req.method, req.path, s.otherOwner, map[string]any{"name": "Mine"}); code != http.StatusForbidden {
			t.Errorf("%s %s by another seller: %d, want 403", req.method, req.path, code)
		}
	}
}

func TestStorefront_Groups(t *testing.T) {
	s := newShop(t)
	bags := s.createGroup("Bags")
	caps := s.createGroup("Caps")
	empty := s.createGroup("Coming soon")
	s.addToGroup(bags.Id, s.bag, s.hidden) // hidden is not on sale
	s.addToGroup(caps.Id, s.cap)
	_ = empty

	w := s.get("/storefront/groups", s.key)
	var groups []struct {
		Name     string `json:"name"`
		Slug     string `json:"slug"`
		Products int    `json:"products"`
	}
	decode(t, w, &groups)
	// Groups with nothing on sale are left out; hidden products aren't counted.
	if w.Code != http.StatusOK || len(groups) != 2 || groups[0].Slug != "bags" || groups[0].Products != 1 || groups[1].Slug != "caps" {
		t.Fatalf("storefront groups: %d %+v", w.Code, groups)
	}
	if body := w.Body.String(); strings.Contains(body, bags.Id) {
		t.Errorf("storefront groups leak ids: %s", body)
	}

	if code, ids := s.storefrontProductIds("/storefront/products?group=bags"); code != http.StatusOK || len(ids) != 1 || ids[0] != s.bag {
		t.Errorf("bags: %d %v", code, ids)
	}
	if code, ids := s.storefrontProductIds("/storefront/products?group=caps"); code != http.StatusOK || len(ids) != 1 || ids[0] != s.cap {
		t.Errorf("caps: %d %v", code, ids)
	}
	if code, _ := s.storefrontProductIds("/storefront/products?group=nope"); code != http.StatusNotFound {
		t.Errorf("unknown group: %d, want 404", code)
	}
	if code, _ := s.storefrontProductIds("/storefront/products?group=bags&ungrouped=true"); code != http.StatusBadRequest {
		t.Errorf("group and ungrouped: %d, want 400", code)
	}

	// Another store's key never sees these groups.
	w = s.get("/storefront/groups", s.otherKey)
	var other []struct{}
	decode(t, w, &other)
	if w.Code != http.StatusOK || len(other) != 0 {
		t.Errorf("other store's groups: %d %+v", w.Code, other)
	}
	if w := s.get("/storefront/products?group=bags", s.otherKey); w.Code != http.StatusNotFound {
		t.Errorf("other store, group=bags: %d, want 404", w.Code)
	}
}
