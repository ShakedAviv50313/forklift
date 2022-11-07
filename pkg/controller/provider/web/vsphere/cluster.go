package vsphere

import (
	"errors"
	"github.com/gin-gonic/gin"
	libmodel "github.com/konveyor/forklift-controller/pkg/lib/inventory/model"
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1"
	model "github.com/konveyor/forklift-controller/pkg/controller/provider/model/vsphere"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web/base"
	"net/http"
	"strings"
)

//
// Routes.
const (
	ClusterParam      = "cluster"
	ClusterCollection = "clusters"
	ClustersRoot      = ProviderRoot + "/" + ClusterCollection
	ClusterRoot       = ClustersRoot + "/:" + ClusterParam
)

//
// Cluster handler.
type ClusterHandler struct {
	Handler
	// Selected cluster.
	cluster *model.Cluster
}

//
// Add routes to the `gin` router.
func (h *ClusterHandler) AddRoutes(e *gin.Engine) {
	e.GET(ClustersRoot, h.List)
	e.GET(ClustersRoot+"/", h.List)
	e.GET(ClusterRoot, h.Get)
}

//
// List resources in a REST collection.
// A GET onn the collection that includes the `X-Watch`
// header will negotiate an upgrade of the connection
// to a websocket and push watch events.
func (h ClusterHandler) List(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	if h.WatchRequest {
		h.watch(ctx)
		return
	}
	var err error
	defer func() {
		if err != nil {
			log.Trace(
				err,
				"url",
				ctx.Request.URL)
			ctx.Status(http.StatusInternalServerError)
		}
	}()
	db := h.Collector.DB()
	list := []model.Cluster{}
	options := h.ListOptions(ctx)
	realCluster := libmodel.Eq("variant", "")
	if options.Predicate != nil {
		options.Predicate = libmodel.And(
			options.Predicate,
			realCluster)
	} else {
		options.Predicate = realCluster
	}
	err = db.List(&list, options)
	if err != nil {
		return
	}
	err = h.filter(ctx, &list)
	if err != nil {
		return
	}
	content := []interface{}{}
	pb := PathBuilder{DB: db}
	for _, m := range list {
		r := &Cluster{}
		r.With(&m)
		r.Link(h.Provider)
		r.Path = pb.Path(&m)
		content = append(content, r.Content(h.Detail))
	}

	ctx.JSON(http.StatusOK, content)
}

//
// Get a specific REST resource.
func (h ClusterHandler) Get(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	m := &model.Cluster{
		Base: model.Base{
			ID: ctx.Param(ClusterParam),
		},
	}
	db := h.Collector.DB()
	err := db.Get(m)
	if errors.Is(err, model.NotFound) {
		ctx.Status(http.StatusNotFound)
		return
	}
	if err != nil {
		log.Trace(
			err,
			"url",
			ctx.Request.URL)
		ctx.Status(http.StatusInternalServerError)
		return
	}
	pb := PathBuilder{DB: db}
	r := &Cluster{}
	r.With(m)
	r.Link(h.Provider)
	r.Path = pb.Path(m)
	content := r.Content(model.MaxDetail)

	ctx.JSON(http.StatusOK, content)
}

//
// Watch.
func (h *ClusterHandler) watch(ctx *gin.Context) {
	db := h.Collector.DB()
	err := h.Watch(
		ctx,
		db,
		&model.Cluster{},
		func(in libmodel.Model) (r interface{}) {
			pb := PathBuilder{DB: db}
			m := in.(*model.Cluster)
			cluster := &Cluster{}
			cluster.With(m)
			cluster.Link(h.Provider)
			cluster.Path = pb.Path(m)
			r = cluster
			return
		})
	if err != nil {
		log.Trace(
			err,
			"url",
			ctx.Request.URL)
		ctx.Status(http.StatusInternalServerError)
	}
}

//
// Filter result set.
// Filter by path for `name` query.
func (h *ClusterHandler) filter(ctx *gin.Context, list *[]model.Cluster) (err error) {
	if len(*list) < 2 {
		return
	}
	q := ctx.Request.URL.Query()
	name := q.Get(NameParam)
	if len(name) == 0 {
		return
	}
	if len(strings.Split(name, "/")) < 2 {
		return
	}
	db := h.Collector.DB()
	pb := PathBuilder{DB: db}
	kept := []model.Cluster{}
	for _, m := range *list {
		path := pb.Path(&m)
		if h.PathMatchRoot(path, name) {
			kept = append(kept, m)
		}
	}

	*list = kept

	return
}

//
// REST Resource.
type Cluster struct {
	Resource
	Folder      string      `json:"folder"`
	Networks    []model.Ref `json:"networks"`
	Datastores  []model.Ref `json:"datastores"`
	Hosts       []model.Ref `json:"hosts"`
	DasEnabled  bool        `json:"dasEnabled"`
	DasVms      []model.Ref `json:"dasVms"`
	DrsEnabled  bool        `json:"drsEnabled"`
	DrsBehavior string      `json:"drsBehavior"`
	DrsVms      []model.Ref `json:"drsVms"`
}

//
// Build the resource using the model.
func (r *Cluster) With(m *model.Cluster) {
	r.Resource.With(&m.Base)
	r.Folder = m.Folder
	r.DasEnabled = m.DasEnabled
	r.DrsEnabled = m.DrsEnabled
	r.DrsBehavior = m.DrsBehavior
	r.Networks = m.Networks
	r.Datastores = m.Datastores
	r.Hosts = m.Hosts
	r.DasVms = m.DasVms
	r.DrsVms = m.DasVms
}

//
// Build self link (URI).
func (r *Cluster) Link(p *api.Provider) {
	r.SelfLink = base.Link(
		ClusterRoot,
		base.Params{
			base.ProviderParam: string(p.UID),
			ClusterParam:       r.ID,
		})
}

//
// As content.
func (r *Cluster) Content(detail int) interface{} {
	if detail == 0 {
		return r.Resource
	}

	return r
}
