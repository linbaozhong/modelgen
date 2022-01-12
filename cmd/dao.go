package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

var dao_str = `
package dao

import (
{{if .HasPrimaryKey}}
	{{if .HasCache}}"libs/utils"
	"internal/cache/redis"
	"fmt"
	"time"
	"internal/conf"{{end}}
{{end}}
	"internal/log"
	"libs/types"
	"{{.ModulePath}}"
	"{{.ModulePath}}/table"
)

type {{lower .StructName}} struct {}

var (
	{{.StructName}} {{lower .StructName}}
{{if and .HasCache .HasPrimaryKey}}
	{{lower .StructName}}_cache     = redis.NewClient(conf.App.Mode,"{{lower .StructName}}").Expiration({{.CacheData}})
	{{lower .StructName}}_ids_cache = redis.NewClient(conf.App.Mode, "{{lower .StructName}}_ids").Expiration({{.CacheList}})
	{{lower .StructName}}_count_cache = redis.NewClient(conf.App.Mode, "{{lower .StructName}}_count").Expiration({{.CacheList}})
{{end}}
)


{{if .HasPrimaryKey}}
//Insert 新增一条数据
func (p {{lower .StructName}}) Insert(x interface{}, bean *models.{{.StructName}}, cols ...string) (int64,error) {
	db := getDB(x, table.{{.StructName}}.TableName)

	if len(cols) > 0 {
		db.Cols(cols...)
	}

	i64, e := db.InsertOne(bean)
	if e != nil {
		log.Logs.DBError(db, e)
	}
{{if .HasCache}}	//向下兼容，未来会移除
	if i64 > 0 {
		p.OnListChange(x)
	}
{{end}}
	return i64, e
}

//InsertBatch 批量新增数据
func (p {{lower .StructName}}) InsertBatch(x interface{}, beans []*models.{{.StructName}}, cols ...string) (int64, error) {
	l := len(beans)
	if l == 0 {
		return 0, Param_Missing
	}
	db := getDB(x, table.{{.StructName}}.TableName)

	if len(cols) > 0 {
		db.Cols(cols...)
	}
	ibeans := make([]interface{}, l)
	for i := 0; i < l; i++ {
		ibeans[i] = beans[i]
	}
	i64, e := db.Insert(ibeans...)
	if e != nil {
		log.Logs.DBError(db, e)
	}
{{if .HasCache}}	//向下兼容，未来会移除
	if i64 > 0 {
		p.OnListChange(x)
	}
{{end}}
	return i64, e
}

//Update 根据主键修改一条数据
func (p {{lower .StructName}}) Update(x interface{}, id types.BigUint, bean interface{}) (int64,error) {
	if bean == nil {
		bean = types.Smap{}
	}
	var (
		i64   int64
		e     error
	)

	db := getDB(x, table.{{.StructName}}.TableName)
	db.Where(table.{{.StructName}}.PrimaryKey.Eq(), id).
		Limit(1)
	if build, ok := bean.(table.ISqlBuilder); ok {
		sm := types.Smap{}
		cols, args := build.GetUpdate()
		for i := 0; i < len(cols); i++ {
			sm.Set(cols[i], args[i])
		}
		exprs := build.GetIncr()
		for _, expr := range exprs {
			db.Incr(expr.ColName, expr.Arg)
		}
		exprs = build.GetDecr()
		for _, expr := range exprs {
			db.Decr(expr.ColName, expr.Arg)
		}
		exprs = build.GetExpr()
		for _, expr := range exprs {
			db.SetExpr(expr.ColName, expr.Arg)
		}
		i64, e = db.Update(sm)
	} else {
		i64, e = db.Update(bean)
	}
	if e != nil {
		log.Logs.DBError(db, e)
	}
{{if .HasCache}}
	if i64 > 0 {
		p.OnChange(x, id)
	}
{{end}}
	return i64, e
}

//UpdateBatch 根据cond条件批量修改数据
func (p {{lower .StructName}}) UpdateBatch(x interface{}, cond table.ISqlBuilder, bean interface{}) (int64, error) {
	if bean == nil {
		bean = types.Smap{}
	}
	var (
		i64 int64
		e   error
{{if .HasCache}}
		ids []interface{}
{{end}}
	)
	
	db := getDB(x, table.{{.StructName}}.TableName)
	if cond != nil {
		ids, e = p.IDsNoCache(nil, cond, 0, 0)
		if e != nil || len(ids) == 0 {
			return 0, e
		}
		if cols := cond.GetCols(); len(cols) > 0 {
			db.Cols(cols...)
		}
		if s, args := cond.GetWhere(); s != "" {
			db.Where(s, args...)
		}
		if size, start := cond.GetLimit(); size > 0 {
			db.Limit(size, start)
		}
		exprs := cond.GetIncr()
		for _, expr := range exprs {
			db.Incr(expr.ColName, expr.Arg)
		}
		exprs = cond.GetDecr()
		for _, expr := range exprs {
			db.Decr(expr.ColName, expr.Arg)
		}
		exprs = cond.GetExpr()
		for _, expr := range exprs {
			db.SetExpr(expr.ColName, expr.Arg)
		}
	}
	//
	i64, e = db.Update(bean)
	if e != nil {
		log.Logs.DBError(db, e)
	}
{{if .HasCache}}
	if i64 > 0 {
		p.OnBatchChange(x, ids)
	}
{{end}}
	return i64, e
}

//Delete 根据主键删除一条数据
func (p {{lower .StructName}}) Delete(x interface{}, id types.BigUint) (int64,error) {
	db := getDB(x, table.{{.StructName}}.TableName)

	i64,e := db.Where(table.{{.StructName}}.PrimaryKey.Eq(),id).
		Limit(1).
		Delete()

	if e != nil {
		log.Logs.DBError(db, e)
	}
{{if .HasCache}}
	if i64 > 0 {
		p.OnChange(x, id)
	}
{{end}}
	return i64, e
}

//DeleteBatch 根据cond条件批量删除数据
func (p {{lower .StructName}}) DeleteBatch(x interface{}, cond table.ISqlBuilder) (int64, error) {
	var (
		i64 int64
		e error
{{if .HasCache}}
		ids []interface{}
{{end}}
	)
	db := getDB(x, table.{{.StructName}}.TableName)

	if cond != nil {
		ids, e = p.IDsNoCache(nil, cond, 0, 0)
		if e != nil || len(ids) == 0 {
			return 0, e
		}
		if s, args := cond.GetWhere(); s != "" {
			db.Where(s, args...)
		}
		if size, start := cond.GetLimit(); size > 0 {
			db.Limit(size, start)
		}
	}
	//
	i64, e = db.Delete()
	if e != nil {
		log.Logs.DBError(db, e)
	}
{{if .HasCache}}
	if i64 > 0 {
		p.OnBatchChange(x, ids)
	}
{{end}}
	return i64, e
}
{{if .HasState}}
//SoftDelete 软删除：根据主键删除一条数据，数据表中必须要state字段 -1=软删除
func (p {{lower .StructName}}) SoftDelete(x interface{}, id types.BigUint) (int64,error) {
	db := getDB(x, table.{{.StructName}}.TableName)

	i64,e := db.Where(table.{{.StructName}}.PrimaryKey.Eq(),id).
		Limit(1).
		Update(types.Smap{
			table.{{.StructName}}.State.Name : -1,
		})

	if e != nil {
		log.Logs.DBError(db, e)
	}
{{if .HasCache}}
	if i64 > 0 {
		p.OnChange(x, id)
	}
{{end}}
	return i64, e
}

//SoftDeleteBatch 软删除：根据cond条件批量删除数据，数据表中必须要state字段 -1=软删除
func (p {{lower .StructName}}) SoftDeleteBatch(x interface{}, cond table.ISqlBuilder) (int64, error) {
	var (
		i64 int64
		e error
{{if .HasCache}}
		ids []interface{}
{{end}}
	)
	db := getDB(x, table.{{.StructName}}.TableName)

	if cond != nil {
		ids, e = p.IDsNoCache(nil, cond, 0, 0)
		if e != nil || len(ids) == 0 {
			return 0, e
		}
		if s, args := cond.GetWhere(); s != "" {
			db.Where(s, args...)
		}
		if size, start := cond.GetLimit(); size > 0 {
			db.Limit(size, start)
		}
	}
	//
	i64, e = db.Update(types.Smap{
			table.{{.StructName}}.State.Name : -1,
		})
	if e != nil {
		log.Logs.DBError(db, e)
	}
{{if .HasCache}}
	if i64 > 0 {
		p.OnBatchChange(x, ids)
	}
{{end}}
	return i64, e
}
{{end}}

//Get 根据主键从Cache中获取一条数据
func (p {{lower .StructName}}) Get(x interface{},id types.BigUint) (bool, *models.{{.StructName}}, error) {
{{if .HasCache}}
	bean := models.New{{.StructName}}()
	if id < 1 {
		return false, bean, Err_NoRows
	}

	s, e := {{lower .StructName}}_cache.Client().Get(getContext(x), {{lower .StructName}}_cache.Key(id)).Result()
	if e != nil {
		//redis key不存在
		if e == redis.Err_Key_Not_Found {
			do, e, _ := Sync("{{lower .StructName}}:"+id.String(), func() (interface{}, error) {
				has, m, e := p.GetNoCache(x, id)
				if has {
					return m, nil
				}
				if e != nil {
					return m, e
				}
				return m, Err_NoRows
			})
			if e != nil {
				return false, bean, e
			}
			return true, do.(*models.{{.StructName}}), nil
		}
		log.Logs.Error(e)
		return p.GetNoCache(x, id)
	}
	if s == "" || s == redis.Err_Value_Not_Found {
		return false, bean, Err_NoRows
	}
	e = json.UnmarshalFromString(s, bean)
	if e != nil {
		log.Logs.Error(e)
		return false, bean, e
	}
	return true, bean, nil
{{else}}
	return p.GetNoCache(x,id)
{{end}}
}

//GetNoCache 根据主键从数据库中获取一条数据
func (p {{lower .StructName}}) GetNoCache(x interface{},id types.BigUint, cols ...table.TableField) (bool, *models.{{.StructName}},error) {
	var bean = models.New{{.StructName}}()
	if id < 1 {
		return false, bean, Err_NoRows
	}
	db := getDB(x, table.{{.StructName}}.TableName)
	//
	l := len(cols)
	if l > 0 {
		_cols := make([]string, 0, l)
		for i := 0; i < l; i++ {
			_cols = append(_cols, cols[i].Name)
		}
		db.Cols(_cols...)
	}

	has, e := db.Where(table.{{.StructName}}.PrimaryKey.Eq(),id).Limit(1).
		Get(bean)
	if has {
{{if .HasCache}}	//重置cache
		if l == 0 {
			s, _ := bean.MarshalJSON()
			{{lower .StructName}}_cache.Client().Set(getContext(x), {{lower .StructName}}_cache.Key(id), string(s), {{lower .StructName}}_cache.DueTime())
		}
{{end}}
		return true, bean, nil
	}
	if e != nil {
		log.Logs.DBError(db, e)
	}
	return false, bean, e
}

//IDs 根据cond条件从cache中获取主键slice
func (p {{lower .StructName}}) IDs(x interface{}, cond table.ISqlBuilder, size, index int) ([]interface{}, error) {
{{if .HasCache}}
	if size == 0 {
		size = {{.CacheLimit}}
	}

	if index == 0 {
		index = 1
	}
	//读取cache
	key := {{lower .StructName}}_ids_cache.Key(cond)
	rs, e := {{lower .StructName}}_ids_cache.Client().LRange(getContext(x), key, int64(size*(index-1)), int64(size*index)-1).Result()
	if e != nil {
		log.Logs.Error(e)
		return []interface{}{}, e
	}

	l := len(rs)
	_rs := make([]interface{}, 0, l)
	//如果返回的是空集
	if l == 0 {
		//检查key是否存在
		n, e := {{lower .StructName}}_ids_cache.Client().Exists(getContext(x), key).Result()
		if e != nil {
			log.Logs.Error(e)
			return []interface{}{}, e
		}
		//key不存在，从数据库中读取
		if n == 0 {
			do, e, _ := Sync("{{lower .StructName}}_ids:"+fmt.Sprintf("%+v", cond), func() (interface{}, error) {
				return p.IDsNoCache(x, cond, size, index)
			})
			if e != nil {
				return _rs, e
			}
			return do.([]interface{}), nil
		}
		return p.IDsNoCache(x, cond, size, index)
	}

	for i := 0; i < l; i++ {
		_rs = append(_rs, utils.String2Uint64(rs[i]))
	}
	return _rs, nil
{{else}}
	return p.IDsNoCache(x,cond,size,index)
{{end}}
}

//IDsNoCache 根据cond条件从数据库中获取主键slice
func (p {{lower .StructName}}) IDsNoCache(x interface{}, cond table.ISqlBuilder, size, index int) ([]interface{}, error) {
	ids, e := getColumn(x,table.{{.StructName}}.TableName, table.{{.StructName}}.PrimaryKey, cond, size, index)
{{if .HasCache}}
	if e == nil {
		//重置cache
		key := {{lower .StructName}}_ids_cache.Key(cond)
		_, e = {{lower .StructName}}_ids_cache.Client().Del(getContext(x), key).Result()
		if e != nil {
			log.Logs.Error(e)
			return ids, nil
		}
		_, e = {{lower .StructName}}_ids_cache.Client().RPush(getContext(x), key, ids...).Result()
		if e != nil {
			log.Logs.Error(e)
		}
	}
{{end}}
	return ids, e
}

//GetColumn 根据cond条件从数据库中单列slice
func (p {{lower .StructName}}) GetColumn(x interface{}, col table.TableField, cond table.ISqlBuilder, size, index int) ([]interface{}, error) {
	return getColumn(x,table.{{.StructName}}.TableName, col, cond, size, index)
}

//Sum 对某个字段进行求和
func (p {{lower .StructName}}) Sum(x interface{}, cond table.ISqlBuilder, col table.TableField) (float64, error) {
	db := getDB(x, table.{{.StructName}}.TableName)

	if cond != nil {
		if joins := cond.GetJoin(); len(joins) > 0 {
			for i := 0; i < len(joins); i++ {
				join := joins[i]
				db.Join(join[0], join[1], join[2])
			}
		}
		if s, args := cond.GetWhere(); s != "" {
			db.Where(s, args...)
		}
		if s := cond.GetGroupBy(); s != "" {
			db.GroupBy(s)
		}
		if s := cond.GetHaving(); s != "" {
			db.Having(s)
		}
	}

	sum, e := db.Sum(p, col.Name)
	if e != nil {
		log.Logs.Error(e)
		return 0, e
	}
	return sum, nil
}

//Sums 对某几个字段进行求和
func (p {{lower .StructName}}) Sums(x interface{}, cond table.ISqlBuilder, args ...table.TableField) ([]float64, error) {
	if len(args) == 0 {
		return []float64{}, Param_Missing
	}
	
	cols := make([]string, len(args))
	for i := 0; i < len(args); i++ {
		cols[i] = args[i].Name
	}

	db := getDB(x, table.{{.StructName}}.TableName)

	if cond != nil {
		if joins := cond.GetJoin(); len(joins) > 0 {
			for i := 0; i < len(joins); i++ {
				join := joins[i]
				db.Join(join[0], join[1], join[2])
			}
		}
		if s, args := cond.GetWhere(); s != "" {
			db.Where(s, args...)
		}
		if s := cond.GetGroupBy(); s != "" {
			db.GroupBy(s)
		}
		if s := cond.GetHaving(); s != "" {
			db.Having(s)
		}
	}

	sums, e := db.Sums(p, cols...)
	if e != nil {
		log.Logs.Error(e)
		return []float64{}, e
	}
	return sums, nil
}

//Count 根据cond条件从cache中获取数据总数
func (p {{lower .StructName}}) Count(x interface{}, cond table.ISqlBuilder) (int64, error) {
{{if .HasCache}}
	i, e := {{lower .StructName}}_count_cache.Client().Get(getContext(x), {{lower .StructName}}_count_cache.Key(cond)).Result()
	if e != nil {
		//redis key不存在
		if e == redis.Err_Key_Not_Found {
			do, e, _ := Sync("{{lower .StructName}}_cound:"+fmt.Sprintf("%+v", cond), func() (interface{}, error) {
				return p.CountNoCache(x, cond)
			})
			if e != nil {
				return 0, e
			}
			return do.(int64), e
		}
		log.Logs.Error(e)
		return p.CountNoCache(x, cond)
	}
	return utils.String2Int64(i), nil
{{else}}
	return p.CountNoCache(x,cond)
{{end}}
}


//CoundNoCache 根据cond条件从数据库中获取数据列表
func (p {{lower .StructName}}) CountNoCache(x interface{}, cond table.ISqlBuilder) (int64, error) {
	db := getDB(x, table.{{.StructName}}.TableName)

	if cond != nil {
		if joins := cond.GetJoin(); len(joins) > 0 {
			for i := 0; i < len(joins); i++ {
				join := joins[i]
				db.Join(join[0], join[1], join[2])
			}
		}
		if s, args := cond.GetWhere(); s != "" {
			db.Where(s, args...)
		}
		if s := cond.GetGroupBy(); s != "" {
			db.GroupBy(s)
		}
		if s := cond.GetHaving(); s != "" {
			db.Having(s)
		}
	}
	i64, e := db.Count()
	if e != nil {
		log.Logs.DBError(db, e)
	}
{{if .HasCache}}	//重置cache
	_, e = {{lower .StructName}}_count_cache.Client().Set(getContext(x), {{lower .StructName}}_count_cache.Key(cond), i64, {{lower .StructName}}_count_cache.DueTime()).Result()
	if e != nil {
		log.Logs.Error(e)
	}
{{end}}
	return i64, nil
}

// Gets 根据主键列表从cache中获取一组数据
func (p {{lower .StructName}}) Gets(x interface{}, ids []interface{}) ([]*models.{{.StructName}}, error) {
{{if .HasCache}}
	l := len(ids)
	if l == 0 {
		return []*models.{{.StructName}}{}, nil
	}
	keys := make([]string, 0, l)
	for i := 0; i < l; i++ {
		keys = append(keys, {{lower .StructName}}_cache.Key(ids[i]))
	}
	rs, e := {{lower .StructName}}_cache.Client().MGet(getContext(x), keys...).Result()
	if e != nil {
		log.Logs.Error(e)
		return []*models.{{.StructName}}{}, e
	}

	_ids := make([]interface{}, 0, l) //未命中的key
	list := make([]*models.{{.StructName}}, 0, l)
	for i := 0; i < l; i++ {
		m := rs[i]
		if m == nil {
			_ids = append(_ids, ids[i])
			continue
		}
		mm := models.New{{.StructName}}()
		if e = json.UnmarshalFromString(utils.Interface2String(m), mm); e == nil {
			list = append(list, mm)
		}
	}
	if len(_ids) > 0 {
		do, _, _ := Sync("{{lower .StructName}}_ids:"+fmt.Sprintf("%+v", _ids), func() (interface{}, error) {
			return p.GetsNoCache(x, _ids)
		})
		if _list, ok := do.([]*models.{{.StructName}}); ok {
			if len(_list) > 0 {
				list = append(list, _list...)
			}
		}
	}
	return list, nil
{{else}}
	return p.GetsNoCache(x, ids)
{{end}}
}

// GetsNoCache 根据主键列表从数据库中获取一组数据
func (p {{lower .StructName}}) GetsNoCache(x interface{}, ids []interface{}) ([]*models.{{.StructName}}, error) {
	idsLen := len(ids)
	if idsLen == 0 {
		return []*models.{{.StructName}}{}, nil
	}

	db := getDB(x, table.{{.StructName}}.TableName)

	list := make([]*models.{{.StructName}}, 0)
	e := db.In(table.{{.StructName}}.PrimaryKey.Name, ids...).Limit(idsLen).Find(&list)
	if e != nil {
		log.Logs.DBError(db, e)
		return list, nil
	}
{{if .HasCache}}
	_ids := make([]interface{}, 0, idsLen)
	l := len(list)
	ctx := getContext(x)
	for i := 0; i < l; i++ {
		m := list[i]
		_ids = append(_ids, m.ID)
		mj, _ := m.MarshalJSON()
		_, e = {{lower .StructName}}_cache.Client().Set(ctx, {{lower .StructName}}_cache.Key(m.ID), string(mj), {{lower .StructName}}_cache.DueTime()).Result()
		if e != nil {
			log.Logs.Error(e)
		}
	}

	for i := 0; i < idsLen; i++ {
		if utils.Contains(_ids, ids[i]) {
			continue
		}
		_, e = {{lower .StructName}}_cache.Client().Set(ctx, {{lower .StructName}}_cache.Key(ids[i]), redis.Err_Value_Not_Found, {{lower .StructName}}_cache.DueTime()).Result()
		if e != nil {
			log.Logs.Error(e)
		}
	}

{{end}}
	return list, nil
}

// GetsMap 根据主键列表从cache中获取一组数据，返回一个 map
func (p {{lower .StructName}}) GetsMap(x interface{}, ids []interface{}) (map[types.BigUint]*models.{{.StructName}}, error) {
	if len(ids) == 0 {
		return map[types.BigUint]*models.{{.StructName}}{}, nil
	}
{{if .HasCache}}
	ms, e := p.Gets(x, ids)
{{else}}
	ms, e := p.GetsNoCache(x, ids)
{{end}}
	if e != nil || len(ms) == 0{
		return map[types.BigUint]*models.{{.StructName}}{}, e
	}
	l := len(ms)
	list := make(map[types.BigUint]*models.{{.StructName}}, l)
	for i := 0; i < l; i++ {
		m := ms[i]
		list[types.BigUint(m.{{.PrimaryKeyName}})] = m
	}
	return list, nil
}

//Find 根据cond条件从cache中获取数据列表
func (p {{lower .StructName}}) Find(x interface{}, cond table.ISqlBuilder, size, index int) ([]*models.{{.StructName}}, error) {
{{if .HasCache}}
	ids, e := p.IDs(x,cond,size,index)
	if len(ids) == 0 {
		return []*models.{{.StructName}}{}, e
	}

	return p.Gets(x, ids)
{{else}}
	return p.FindNoCache(x,cond,size,index)
{{end}}
}

//FindNoCache 根据cond条件从数据库中获取数据列表
func (p {{lower .StructName}}) FindNoCache(x interface{}, cond table.ISqlBuilder, size, index int) ([]*models.{{.StructName}}, error) {
	db := getDB(x, table.{{.StructName}}.TableName)

	list := make([]*models.{{.StructName}}, 0)

	if cond == nil {
		if size > 0 {
			if index == 0 {
				index = 1
			}
			db.Limit(size, size*(index-1))
		}
	} else {
		if joins := cond.GetJoin(); len(joins) > 0 {
			for i := 0; i < len(joins); i++ {
				join := joins[i]
				db.Join(join[0], join[1], join[2])
			}
		}
		if cols := cond.GetCols(); len(cols) > 0 {
			db.Cols(cols...)
		}
		if s, args := cond.GetWhere(); s != "" {
			db.Where(s, args...)
		}
		if s := cond.GetGroupBy(); s != "" {
			db.GroupBy(s)
		}
		if s := cond.GetHaving(); s != "" {
			db.Having(s)
		}
		if s := cond.GetOrderBy(); s != "" {
			db.OrderBy(s)
		}
		if size > 0 {
			if index == 0 {
				index = 1
			}
			db.Limit(size, size*(index-1))
		} else if i, start := cond.GetLimit(); i > 0 {
			db.Limit(i, start)
		}
	}

	e := db.Find(&list)
	if e != nil {
		log.Logs.DBError(db, e)
	}
	return list, nil
}

//FindMap 根据cond条件从cache中获取数据列表，返回一个 map
func (p {{lower .StructName}}) FindMap(x interface{}, cond table.ISqlBuilder, size, index int) (map[types.BigUint]*models.{{.StructName}}, error) {
{{if .HasCache}}
	ids, e := p.IDs(x,cond,size,index)
	if len(ids) == 0 {
		return map[types.BigUint]*models.{{.StructName}}{}, e
	}

	return p.GetsMap(x, ids)
{{else}}
	ms, e := p.FindNoCache(x,cond,size,index)
	if e != nil || len(ms) == 0{
		return map[types.BigUint]*models.{{.StructName}}{}, e
	}
	l := len(ms)
	list := make(map[types.BigUint]*models.{{.StructName}}, l)
	for i := 0; i < l; i++ {
		m := ms[i]
		list[types.BigUint(m.{{.PrimaryKeyName}})] = m
	}
	return list, nil
{{end}}
}

//FindOne 根据cond条件从cache中获取一条数据
func (p {{lower .StructName}}) FindOne(x interface{}, cond table.ISqlBuilder) (bool, *models.{{.StructName}}, error) {
	if cond != nil {
		cond.Limit(1)
	}
	f, e := p.Find(x, cond, 1, 1)
	if len(f) > 0 {
		return true, f[0],nil
	}
	return false, models.New{{.StructName}}(), e
}

//FindOneNoCache 根据cond条件从数据库中获取一条数据
func (p {{lower .StructName}}) FindOneNoCache(x interface{}, cond table.ISqlBuilder) (bool, *models.{{.StructName}},error) {
	if cond != nil {
		cond.Limit(1)
	}
	f, e := p.FindNoCache(x, cond, 1, 1)
	if len(f) > 0 {
		return true, f[0],nil
	}
	return false, models.New{{.StructName}}(), e
}

//FindAndCound
func (p {{lower .StructName}}) FindAndCount(x interface{}, cond table.ISqlBuilder, size, index int) (i64 int64, ms []*models.{{.StructName}}, e error) {
	i64, e = p.Count(x, cond)
	if e != nil || i64 == 0 {
		return i64, nil, e
	}
	ms, e = p.Find(x, cond, size, index)
	return
}

//QueryInterfaces 多表连接查询
func (p {{lower .StructName}}) QueryInterfaces(x interface{}, cond table.ISqlBuilder) ([]map[string]interface{}, error) {
	db := getDB(x, table.{{.StructName}}.TableName)
	sm, e := queryInterfaces(db, cond.Table(table.{{.StructName}}.TableName))
	if e != nil {
		log.Logs.DBError(db, e)
	}
	return sm, e
}

//Exists 是否存在符合条件cond的记录
func (p {{lower .StructName}}) Exists(x interface{}, cond table.ISqlBuilder) (bool, error) {
	db := getDB(x, table.{{.StructName}}.TableName)

	if cond != nil {
		if s, args := cond.GetWhere(); s != "" {
			db.Where(s, args...)
		}
	}

	has, e := db.Limit(1).Exist()
	if e != nil {
		log.Logs.DBError(db, e)
	}
	return has, e
}

{{if .HasCache}}
//OnChange
func (p {{lower .StructName}}) OnChange(x interface{}, id types.BigUint) {
	{{lower .StructName}}_cache.Remove(getContext(x), id)
}

//OnBatchChange
func (p {{lower .StructName}}) OnBatchChange(x interface{}, ids []interface{}) {
	if len(ids) == 0 {
		return 
	}
	{{lower .StructName}}_cache.Remove(getContext(x), ids...)
}
//OnListChange
func (p {{lower .StructName}}) OnListChange(x interface{}, cond ...table.ISqlBuilder) {
	ctx := getContext(x)
	if len(cond) == 0 {
		{{lower .StructName}}_ids_cache.Empty(ctx)
		{{lower .StructName}}_count_cache.Empty(ctx)
		return
	}
	{{lower .StructName}}_ids_cache.Remove(ctx, cond[0])
	{{lower .StructName}}_count_cache.Remove(ctx, cond[0])
}

func (p {{lower .StructName}})Cache() *redis.RedisBroker {
	return {{lower .StructName}}_cache
}

func (p {{lower .StructName}})IDsCache() *redis.RedisBroker {
	return {{lower .StructName}}_ids_cache
}

func (p {{lower .StructName}})CountCache() *redis.RedisBroker {
	return {{lower .StructName}}_count_cache
}

//SliceToJSON slice转json
func (p {{lower .StructName}}) SliceToJSON(sls []*models.{{.StructName}},cols...table.TableField) []types.Smap {
	sl := len(sls)
	if sl == 0 {
		return []types.Smap{}
	}
	var (
		sm types.Smap
		m  map[string]interface{}
	)
	ms := make([]types.Smap, 0, sl)
	if len(cols) == 0 {
		l := len(table.{{.StructName}}.ColumnNames)
		for i := 0; i < sl; i++ {
			m = sls[i].ToMap()
			sm = make(types.Smap, l)
			for _, cn := range table.{{.StructName}}.ColumnNames {
				sm.Set(table.{{.StructName}}.ColumnName2Json[cn], m[cn])
			}
			ms = append(ms, sm)
		}
		return ms
	}
	for i := 0; i < sl; i++ {
		m = sls[i].ToMap()
		l := len(cols)
		sm = make(types.Smap, l)
		for i := 0; i < l; i++ {
			col := cols[i]
			sm.Set(table.{{.StructName}}.ColumnName2Json[col.Name], m[col.Name])
		}

		ms = append(ms, sm)
	}
	return ms
}

{{end}}
{{end}}

`

func (d *TempData) writeToDao(fileName string) error {
	if !d.HasPrimaryKey {
		return nil
	}

	var buf bytes.Buffer
	funcMap := template.FuncMap{
		"lower": strings.ToLower,
	}

	err := template.Must(template.New("daoTpl").Funcs(funcMap).Parse(dao_str)).Execute(&buf, d)
	if err != nil {
		showError(err)
		return err
	}

	absPath, _ := filepath.Abs(fileName)
	fileName = filepath.Join(filepath.Dir(absPath), "dao", d.StructName+"_dao.go")

	var (
		f *os.File
	)

	f, err = os.Create(fileName)

	if err != nil {
		showError(err.Error())
		return err
	}
	defer f.Close()

	_, err = f.Write(buf.Bytes())
	if err != nil {
		showError(err)
		return err
	}

	return nil
}
