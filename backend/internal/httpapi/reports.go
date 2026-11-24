package httpapi

import (
	"encoding/json"
	"github.com/gofiber/fiber/v2"
	"saturnita/pulse/internal/identity"
	"strings"
	"time"
)

func interval(c *fiber.Ctx) (from, to time.Time, err error) {
	to = time.Now().UTC()
	if s := c.Query("to"); s != "" {
		to, err = time.Parse(time.RFC3339, s)
		if err != nil {
			return from, to, identity.ErrInvalid
		}
	}
	from = to.Add(-7 * 24 * time.Hour)
	if s := c.Query("from"); s != "" {
		from, err = time.Parse(time.RFC3339, s)
	}
	if err != nil || !from.Before(to) || to.Sub(from) > 180*24*time.Hour {
		return from, to, identity.ErrInvalid
	}
	return
}
func breakdown(column string) string {
	return "(select coalesce(jsonb_agg(to_jsonb(x) order by value desc,key),'[]') from (select coalesce(nullif(" + column + ",''),'—') key,count(*) value from current_rows group by 1 order by 2 desc,1 limit 20) x)"
}
func (p *product) report(c *fiber.Ctx) error {
	from, to, e := interval(c)
	if e != nil {
		return e
	}
	ownerID := owner(c)
	id := c.Params("id")
	site := strings.Contains(c.Path(), "/sites/")
	ctx := c.UserContext()
	zone := "UTC"
	if id != "" {
		if _, e = pathID(c); e != nil {
			return e
		}
		sql := "select 'UTC' from links where user_id=$1 and id=$2"
		if site {
			sql = "select timezone from sites where user_id=$1 and id=$2"
		}
		if e = p.db.QueryRow(ctx, sql, ownerID, id).Scan(&zone); e != nil {
			return e
		}
	}
	gran := c.Query("granularity")
	if gran == "" {
		gran = "day"
		if to.Sub(from) <= 24*time.Hour {
			gran = "hour"
		}
	}
	if gran != "hour" && gran != "day" {
		return identity.ErrInvalid
	}
	base := `select a.occurred_at t,a.referrer,a.source,a.medium,a.campaign,a.device,a.browser,a.os,a.visitor_hash,a.is_bot,l.slug name,d.hostname||'/'||l.slug path,'click' kind from link_clicks a join links l on l.id=a.link_id join domains d on d.id=l.domain_id where l.user_id=$1 and ($2='' or l.id=nullif($2,'')::uuid) and a.occurred_at >= $3::timestamptz-($4::timestamptz-$3) and a.occurred_at<$4`
	metric := "true"
	if site {
		base = `select a.received_at t,a.referrer,a.source,a.medium,a.campaign,a.device,a.browser,a.os,a.visitor_hash,false is_bot,a.name,a.path,a.kind from web_events a join sites s on s.id=a.site_id where s.user_id=$1 and s.id=nullif($2,'')::uuid and a.received_at >= $3::timestamptz-($4::timestamptz-$3) and a.received_at<$4`
		metric = "kind='pageview'"
	}
	sql := "with all_rows as (" + base + "), current_rows as (select * from all_rows where t >= $3), prior as (select count(*) n from all_rows where t<$3 and " + metric + "), counts as (select count(*) filter(where " + metric + ") n from current_rows) select jsonb_build_object(" + strings.Join([]string{
		"'active_monitors',(select count(*) from monitors where enabled and user_id=$1)", "'monitor_alerts',(select coalesce(jsonb_agg(to_jsonb(m)),'[]') from (select id,name,status from monitors where user_id=$1 and status in ('down','degraded') order by status desc,name limit 20)m)",
		"'total',(select n from counts)", "'previous',(select n from prior)", "'change_percent',(select round((counts.n-prior.n)*100.0/nullif(prior.n,0),1) from counts,prior)",
		"'daily_unique_visitors_sum',(select count(distinct ((t at time zone 'UTC')::date,visitor_hash)) from current_rows)", "'bots',(select count(*) from current_rows where is_bot)",
		"'series',(select coalesce(jsonb_agg(to_jsonb(x) order by bucket),'[]') from (select bucket,coalesce(n,0) value from generate_series(date_trunc($5,$3::timestamptz,$6),date_trunc($5,$4::timestamptz-interval '1 microsecond',$6),('1 '||$5)::interval,$6) bucket left join (select date_trunc($5,t,$6) b,count(*) n from current_rows where " + metric + " group by 1) a on a.b=bucket where (select n from counts)>0) x)",
		"'devices'," + breakdown("device"), "'sources'," + breakdown("referrer"), "'campaigns'," + breakdown("campaign"), "'utm_sources'," + breakdown("source"), "'browsers'," + breakdown("browser"), "'systems'," + breakdown("os"), "'pages'," + breakdown("path"), "'events'," + breakdown("name"),
		"'recent',(select coalesce(jsonb_agg(to_jsonb(x) order by t desc),'[]') from (select t,name,path,device,referrer from current_rows order by t desc limit 20) x)",
		"'active_links',(select count(*) from links where user_id=$1 and status='active' and (expires_at is null or expires_at>now()) and (click_limit is null or click_count<click_limit))", "'sites',(select count(*) from sites where user_id=$1)", "'timezone',$6::text"}, ",") + ")"
	// Top pages on websites rank views only. Custom event totals are counted separately.
	if site {
		sql = strings.Replace(sql, breakdown("path"), strings.Replace(breakdown("path"), "from current_rows group", "from current_rows where kind='pageview' group", 1), 1)
		sql = strings.Replace(sql, breakdown("name"), strings.Replace(breakdown("name"), "from current_rows group", "from current_rows where kind='custom' group", 1), 1)
	}
	var data json.RawMessage
	if e = p.db.QueryRow(ctx, sql, ownerID, id, from, to, gran, zone).Scan(&data); e != nil {
		return e
	}
	return c.JSON(data)
}
func (p *product) clicks(c *fiber.Ctx) error {
	id, e := pathID(c)
	if e != nil {
		return e
	}
	var ok int
	if e = p.db.QueryRow(c.UserContext(), "select 1 from links where user_id=$1 and id=$2", owner(c), id).Scan(&ok); e != nil {
		return e
	}
	return list(c, p.db, `select to_jsonb(r) from (select a.occurred_at,a.referrer,a.source,a.medium,a.campaign,a.device,a.browser,a.os,a.is_bot from link_clicks a join links l on a.link_id=l.id where l.user_id=$1 and l.id=$2 order by occurred_at desc limit 100) r`, owner(c), id)
}
