package ngago

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/astaxie/beego"
)

type RESTController interface {
	NewRepo() Repository
	Id(entity interface{}) int64
}

/*
Controllers can implement this interface as a simple authorization mechanism. The profile should
be provided by a filter as a Ctx.Input data entry.

When implemented, the AccessControl method will receive the resource and authentication information
and must return true when the access is allowed, or false otherwise
*/
type AuthenticatedController interface {
	AccessControl(controller, action, url, profile string) bool
}

type BaseController struct {
	beego.Controller
}

func (c *BaseController) getData(name string) string {
	if v, ok := c.Ctx.Input.GetData(name).(string); ok {
		return v
	}
	return ""
}

func (c *BaseController) SendError(code, message string) {
	c.Data["message"] = message
	c.Abort(code)
}

type BaseRESTController struct {
	BaseController
	repo Repository
}

func (c *BaseRESTController) Prepare() {
	c.repo = c.AppController.(RESTController).NewRepo()
	authController, ok := c.AppController.(AuthenticatedController)
	if !ok {
		return
	}
	controller, action := c.GetControllerAndAction()
	url := c.Ctx.Request.URL.Path
	user := c.getData("user")
	profile := c.getData("profile")
	if !authController.AccessControl(controller, action, url, profile) {
		beego.Warn(fmt.Sprintf("Access denied! User: %s, Profile: %s, URL: %s", user, profile, url))
		c.SendError("401", "Access denied!")
	}
}

func (c *BaseRESTController) Repo() Repository {
	return c.repo
}

func (c *BaseRESTController) Get() {
	var id int64
	c.Ctx.Input.Bind(&id, ":id")
	if id != 0 {
		entity := c.repo.NewInstance()
		err := c.repo.Read(id, entity)
		if err == ErrNotFound {
			msg := fmt.Sprintf("%s %d not found", c.EntityName(), id)
			beego.Warn(msg)
			c.SendError("404", msg)
		}
		if err != nil {
			beego.Error(fmt.Sprintf("Error reading %ss: %v", c.EntityName(), err))
			c.SendError("500", err.Error())
		}
		c.Data["json"] = &entity
	} else {
		options := c.parseOptions()
		entities := c.repo.NewSlice()
		err := c.repo.ReadAll(entities, options)
		if err != nil {
			beego.Error(fmt.Sprintf("Error reading %s: %v", c.EntityName(), err))
			c.SendError("500", err.Error())
		}
		count, _ := c.repo.Count(options)
		c.Ctx.Output.Header("X-Total-Count", strconv.FormatInt(count, 10))
		c.Data["json"] = &entities
	}
	c.ServeJSON()
}

func (c *BaseRESTController) Put() {
	entity := c.repo.NewInstance()
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, entity); err != nil {
		beego.Error(fmt.Sprintf("Error parsing %s %#v: %v", c.EntityName(), string(c.Ctx.Input.RequestBody), err))
		c.SendError("422", err.Error())
	}
	id := c.GetId(entity)
	err := c.repo.Update(entity)
	if err == ErrNotFound {
		msg := fmt.Sprintf("%s %d not found", c.EntityName(), id)
		beego.Warn(msg)
		c.SendError("404", msg)
	}
	if err != nil {
		beego.Error(fmt.Sprintf("Error updating %s %#v: %v", c.EntityName(), entity, err))
		c.SendError("500", err.Error())
	}
	c.Data["json"] = &entity
	c.ServeJSON()
}

func (c *BaseRESTController) Post() {
	entity := c.repo.NewInstance()
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, entity); err != nil {
		beego.Error(fmt.Sprintf("Error parsing %s %#v: %v", c.EntityName(), string(c.Ctx.Input.RequestBody), err))
		c.SendError("422", err.Error())
	}
	id, err := c.repo.Save(entity)
	if err != nil {
		beego.Error(fmt.Sprintf("Error creating %s %#v: %v", c.EntityName(), entity, err))
		c.SendError("500", err.Error())
	}
	c.Data["json"] = map[string]int64{"id": id}
	c.ServeJSON()
}

func (c *BaseRESTController) Delete() {
	var id int64
	c.Ctx.Input.Bind(&id, ":id")
	err := c.repo.Delete(id)
	if err == ErrNotFound {
		msg := fmt.Sprintf("%s %d not found", c.EntityName(), id)
		beego.Warn(msg)
		c.SendError("404", msg)
	}
	if err != nil {
		beego.Error(fmt.Sprintf("Error deleting %s %d: %v", c.EntityName(), id, err))
		c.SendError("500", err.Error())
	}
	c.Data["json"] = map[string]string{}
	c.ServeJSON()
}

func (c *BaseRESTController) GetId(entity interface{}) int64 {
	return c.AppController.(RESTController).Id(entity)
}

func (c *BaseRESTController) EntityName() string {
	return c.repo.EntityName()
}

func (c *BaseRESTController) parseFilters() map[string]interface{} {
	var filterStr = c.GetString("_filters")
	filters := make(map[string]interface{})
	if filterStr != "" {
		filterStr, _ = url.QueryUnescape(filterStr)
		if err := json.Unmarshal([]byte(filterStr), &filters); err != nil {
			beego.Warn("Invalid filter specification:", filterStr, "-", err.Error())
		}
	}
	for k, v := range c.Input() {
		if strings.HasPrefix(k, "_") {
			continue
		}
		filters[k] = v[0]
	}
	return filters
}

func (c *BaseRESTController) parseOptions() QueryOptions {
	perPage, page := 0, 1
	c.Ctx.Input.Bind(&page, "_page")
	c.Ctx.Input.Bind(&perPage, "_perPage")

	sortField := c.Input().Get("_sortField")
	sortDir := c.Input().Get("_sortDir")

	return QueryOptions{
		Sort:    sortField,
		Order:   strings.ToLower(sortDir),
		Offset:  (page - 1) * perPage,
		Max:     perPage,
		Filters: c.parseFilters(),
	}
}
