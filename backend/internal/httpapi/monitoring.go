package httpapi

import (
	"github.com/gofiber/fiber/v2"
	"saturnita/pulse/internal/monitoring"
)

func (p *product) monitors(api fiber.Router) {
	s := monitoring.Store{DB: p.db}
	base := "/monitors"
	api.Get(base, func(c *fiber.Ctx) error {
		m, e := s.List(c.UserContext(), owner(c))
		if e != nil {
			return e
		}
		return c.JSON(fiber.Map{"items": m})
	})
	api.Get(base+"/:id", func(c *fiber.Ctx) error {
		id, e := pathID(c)
		if e != nil {
			return e
		}
		m, e := s.Get(c.UserContext(), owner(c), id)
		if e != nil {
			return e
		}
		return c.JSON(m)
	})
	save := func(c *fiber.Ctx) error {
		id := c.Params("id")
		if id != "" {
			if _, e := pathID(c); e != nil {
				return e
			}
		}
		m, e := s.Save(c.UserContext(), owner(c), id, func(d *monitoring.Config) error { return decode(c, d) })
		if e != nil {
			return e
		}
		if id == "" {
			c.Status(201)
		}
		return c.JSON(m)
	}
	api.Post(base, save)
	api.Patch(base+"/:id", save)
	for _, a := range []string{"pause", "resume", "check", "delete"} {
		run := func(c *fiber.Ctx) error {
			id, e := pathID(c)
			if e != nil {
				return e
			}
			if e = s.Action(c.UserContext(), owner(c), id, a); e != nil {
				return e
			}
			if a == "check" {
				return c.SendStatus(202)
			}
			return c.SendStatus(204)
		}
		if a == "delete" {
			api.Delete(base+"/:id", run)
		} else {
			api.Post(base+"/:id/"+a, run)
		}
	}
	for _, v := range []string{"checks", "analytics", "incidents"} {
		api.Get(base+"/:id/"+v, func(c *fiber.Ctx) error {
			id, e := pathID(c)
			if e != nil {
				return e
			}
			from, to, e := interval(c)
			if e != nil {
				return e
			}
			out, e := s.Report(c.UserContext(), owner(c), id, v, from, to)
			if e != nil {
				return e
			}
			return c.JSON(out)
		})
	}
}
