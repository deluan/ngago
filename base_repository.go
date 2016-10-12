package ngago

import (
	"reflect"
	"strconv"
	"strings"

	"github.com/astaxie/beego/orm"
)

var ErrNotFound = orm.ErrNoRows

type QueryOptions struct {
	Sort    string
	Order   string
	Offset  int
	Max     int
	Filters map[string]interface{}
}

type Repository interface {
	// These methods are provided by the BaseRepository struct
	Count(options ...QueryOptions) (int64, error)
	Read(id int64, data interface{}) error
	ReadAll(dataSet interface{}, options ...QueryOptions) error
	Save(p interface{}) (int64, error)
	Update(p interface{}, cols ...string) error
	Delete(id int64) error
	EntityName() string
	NewSlice() interface{}
	NewInstance() interface{}

	// TODO Split into different interfaces
	// These methods can be overriden by subclasses to manipulate the queries used by Read and ReadAll
	One(qs orm.QuerySeter, data interface{}) error
	All(qs orm.QuerySeter, dataSet interface{}) (int64, error)
}

type FilterFunc func(qs orm.QuerySeter, field, value string) orm.QuerySeter

type BaseRepository struct {
	Orm orm.Ormer

	self         Repository
	table        string
	filterMap    map[string]FilterFunc
	instanceType reflect.Type
	sliceType    reflect.Type
}

func (r *BaseRepository) Init(table string, instance interface{}, ormer ...orm.Ormer) {
	r.self = r
	r.table = table
	r.filterMap = make(map[string]FilterFunc)
	r.instanceType = reflect.TypeOf(instance)
	r.sliceType = reflect.SliceOf(r.instanceType)
	if len(ormer) > 0 {
		r.Orm = ormer[0]
	} else {
		r.Orm = orm.NewOrm()
	}
}

func (r *BaseRepository) AddFilter(field string, function FilterFunc) {
	r.filterMap[field] = function
}

func (r *BaseRepository) NewInstance() interface{} {
	return reflect.New(r.instanceType).Interface()
}

func (r *BaseRepository) NewSlice() interface{} {
	slice := reflect.MakeSlice(r.sliceType, 0, 0)
	x := reflect.New(slice.Type())
	x.Elem().Set(slice)
	return x.Interface()
}

func (r *BaseRepository) One(qs orm.QuerySeter, data interface{}) error {
	return qs.RelatedSel().One(data)
}

func (r *BaseRepository) All(qs orm.QuerySeter, dataSet interface{}) (int64, error) {
	return qs.RelatedSel().All(dataSet)
}

func (r *BaseRepository) EntityName() string {
	return r.table
}

func (r *BaseRepository) Read(id int64, data interface{}) error {
	qs := r.Orm.QueryTable(r.table).Filter("Id", id)
	err := r.self.One(qs, data)
	return err
}

func (r *BaseRepository) Count(options ...QueryOptions) (int64, error) {
	qs := r.Orm.QueryTable(r.table)
	qs = r.AddFilters(qs, options)
	return qs.Count()
}

func (r *BaseRepository) ReadAll(dataSet interface{}, options ...QueryOptions) error {
	qs := r.Orm.QueryTable(r.table)
	qs = r.AddOptions(qs, options)
	qs = r.AddFilters(qs, options)
	_, err := r.self.All(qs, dataSet)
	return err
}

func (r *BaseRepository) Save(p interface{}) (int64, error) {
	return r.Orm.Insert(p)
}

func (r *BaseRepository) Update(p interface{}, cols ...string) error {
	count, err := r.Orm.Update(p, cols...)
	if err != nil {
		return err
	}
	if count == 0 {
		return ErrNotFound
	}
	return err
}

func (r *BaseRepository) Delete(id int64) error {
	_, err := r.Orm.QueryTable(r.table).Filter("Id", id).Delete()
	return err
}

func (r *BaseRepository) AddOptions(qs orm.QuerySeter, options []QueryOptions) orm.QuerySeter {
	if len(options) == 0 {
		return qs
	}
	opt := options[0]
	sort := strings.Split(opt.Sort, ",")
	reverse := strings.ToLower(opt.Order) == "desc"
	for i, s := range sort {
		s = strings.TrimSpace(s)
		if reverse {
			if s[0] == '-' {
				s = strings.TrimPrefix(s, "-")
			} else {
				s = "-" + s
			}
		}
		sort[i] = strings.Replace(s, ".", "__", -1)
	}
	if opt.Sort != "" {
		qs = qs.OrderBy(sort...)
	}
	if opt.Max > 0 {
		qs = qs.Limit(opt.Max)
	}
	if opt.Offset > 0 {
		qs = qs.Offset(opt.Offset)
	}
	return qs
}

func (r *BaseRepository) AddFilters(qs orm.QuerySeter, options []QueryOptions) orm.QuerySeter {
	if len(options) != 0 {
		for f, v := range options[0].Filters {
			fn := strings.Replace(f, ".", "__", -1)
			var s string
			if i, ok := v.(float64); ok {
				s = strconv.FormatFloat(i, 'f', -1, 64)
			} else {
				s = v.(string)
			}

			if ff, ok := r.filterMap[f]; ok {
				qs = ff(qs, fn, s)
			} else if strings.HasSuffix(fn, "Id") || strings.HasSuffix(fn, "__id") {
				qs = IdFilter(qs, fn, s)
			} else {
				qs = StartsWithFilter(qs, fn, s)
			}
		}
	}
	return qs
}

func IdFilter(qs orm.QuerySeter, field, value string) orm.QuerySeter {
	field = strings.TrimSuffix(field, "Id") + "__id"
	id, _ := strconv.Atoi(value)
	return qs.Filter(field, id)
}

func BooleanFilter(qs orm.QuerySeter, field, value string) orm.QuerySeter {
	return qs.Filter(field, value == "true")
}

func StartsWithFilter(qs orm.QuerySeter, field, value string) orm.QuerySeter {
	return qs.Filter(field+"__istartswith", value)
}

func ContainsWithFilter(qs orm.QuerySeter, field, value string) orm.QuerySeter {
	return qs.Filter(field+"__icontains", value)
}
