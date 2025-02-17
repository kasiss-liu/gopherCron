package router

import (
	"net/http"

	"github.com/holdno/gopherCron/app"
	"github.com/holdno/gopherCron/cmd/service/controller"
	"github.com/holdno/gopherCron/cmd/service/controller/etcd_func"
	"github.com/holdno/gopherCron/cmd/service/controller/log_func"
	"github.com/holdno/gopherCron/cmd/service/controller/project_func"
	"github.com/holdno/gopherCron/cmd/service/controller/system"
	"github.com/holdno/gopherCron/cmd/service/controller/user_func"
	"github.com/holdno/gopherCron/cmd/service/middleware"
	"github.com/holdno/gopherCron/common"
	"github.com/holdno/gopherCron/config"
	"github.com/holdno/gopherCron/pkg/metrics"
	"github.com/holdno/gopherCron/utils"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spacegrower/watermelon/infra/wlog"

	"github.com/gin-contrib/pprof"
	"github.com/gin-gonic/gin"
)

func prometheusHandler(r *prometheus.Registry) gin.HandlerFunc {
	h := promhttp.InstrumentMetricHandler(
		r, promhttp.HandlerFor(r, promhttp.HandlerOpts{}),
	)

	return func(c *gin.Context) {
		h.ServeHTTP(c.Writer, c.Request)
	}
}

func SetupRoute(srv app.App, r *gin.Engine, conf *config.DeployConf) {
	r.Use(gin.Recovery())
	if utils.DebugMode() {
		wlog.Info("debug mode will open pprof tools")
		pprof.Register(r)
	}
	r.Use(func(c *gin.Context) {
		c.Set(common.APP_KEY, srv)
	})
	r.NoRoute(func(c *gin.Context) {
		c.String(http.StatusOK, "no router found")
	})
	r.NoMethod(func(c *gin.Context) {
		c.String(http.StatusOK, "no method found")
	})
	r.Use(middleware.CrossDomain())
	r.Use(middleware.BuildResponse())
	r.Use(metrics.Middleware(srv.Metrics()))
	r.GET("/healthy", func(c *gin.Context) {
		c.String(http.StatusOK, "healthy")
	})
	r.GET("/metrics", prometheusHandler(srv.Metrics().Registry()))

	api := r.Group("/api/v1")
	{
		api.GET("/version", system.GetVersion)
		user := api.Group("/user")
		{
			user.POST("/login", user_func.Login)
			user.Use(middleware.TokenVerify())
			user.GET("/info", user_func.GetUserInfo)
			user.POST("/change_password", user_func.ChangePassword)
			user.POST("/create", user_func.CreateUser)
			user.POST("/delete", user_func.DeleteUser)
			user.GET("/list", user_func.GetUserList)
		}

		oidc := api.Group("/oidc")
		{
			oidc.POST("/login", user_func.OIDCLogin)
			oidc.GET("/auth_url", user_func.OIDCAuthURL)
		}

		webhook := api.Group("/webhook")
		{
			webhook.Use(middleware.TokenVerify())
			webhook.POST("/create", controller.CreateWebHook)
			webhook.POST("/delete", controller.DeleteWebHook)
			webhook.GET("/list", controller.GetWebHookList)
			webhook.GET("/info", controller.GetWebHook)
		}

		cron := api.Group("/crontab")
		{
			cron.Use(middleware.TokenVerify())
			cron.POST("/save", etcd_func.SaveTask)
			cron.POST("/delete", etcd_func.DeleteTask)
			cron.GET("/list", etcd_func.GetTaskList)
			cron.POST("/kill", etcd_func.KillTask)
			cron.POST("/execute", etcd_func.ExecuteTask)
			cron.GET("/client_list", etcd_func.GetClientList)
			cron.POST("/monitor", etcd_func.GetWorkerListInfo)
			service := cron.Group("/tmp")
			{
				service.POST("/execute", etcd_func.TmpExecute)
			}
		}

		temporaryTask := api.Group("/temporary_task")
		{
			temporaryTask.Use(middleware.TokenVerify())
			temporaryTask.POST("/create", controller.CreateTemporaryTask)
			temporaryTask.POST("/delete", controller.DeleteTemporaryTask)
			temporaryTask.GET("/list", controller.GetTemporaryTaskList)
		}

		workflow := api.Group("/workflow")
		{
			workflow.Use(middleware.TokenVerify())
			workflow.POST("/create", controller.CreateWorkflow)
			workflow.POST("/delete", controller.DeleteWorkflow)
			workflow.POST("/update", controller.UpdateWorkflow)
			workflow.GET("/list", controller.GetWorkflowList)
			workflow.GET("/detail", controller.GetWorkflow)
			workflow.POST("/start", controller.StartWorkflow)
			workflow.POST("/kill", controller.KillWorkflow)
			manage := workflow.Group("/manage")
			{
				manage.POST("/add_user", controller.WorkflowAddUser)
				manage.POST("/remove_user", controller.WorkflowRemoveUser)
				manage.GET("/users", controller.GetUsersByWorkflow)
			}
			task := workflow.Group("/task")
			{
				task.POST("/schedule/create", controller.CreateWorkflowSchedulePlan)
				task.GET("/list", controller.GetWorkflowTaskList)
			}
			log := workflow.Group("/log")
			{
				log.GET("/list", controller.GetWorkflowLogList)
				log.POST("/clear", controller.ClearWorkflowLog)
			}
		}

		worker := api.Group("/client")
		{
			worker.Use(middleware.TokenVerify())
			worker.GET("/list", etcd_func.GetWorkerListInfo)
			worker.POST("/reload/config", etcd_func.ReloadConfig)
		}

		registry := api.Group("/registry")
		{
			registry.POST("/remove")
		}

		project := api.Group("/project")
		{
			project.Use(middleware.TokenVerify())
			project.POST("/create", project_func.Create)
			project.GET("/list", project_func.GetUserProjects)
			project.POST("/update", project_func.Update)
			project.POST("/delete", project_func.DeleteOne)
			project.GET("/users", user_func.GetUsersUnderTheProject)
			project.POST("/remove_user", project_func.RemoveUser)
			project.POST("/add_user", project_func.AddUser)
			workflow := project.Group("/workflow")
			{
				workflow.GET("/task/list", project_func.GetProjectWorkflowTasks)
				workflow.POST("/task/create", project_func.CreateProjectWorkflowTask)
				workflow.POST("/task/delete", project_func.DeleteProjectWorkflowTask)
				workflow.POST("/task/update", project_func.UpdateProjectWorkflowTask)
			}
		}

		log := api.Group("/log")
		{
			log.Use(middleware.TokenVerify())
			log.GET("/list", log_func.GetList)
			log.GET("/detail", log_func.GetLogDetail)
			log.POST("/clean", log_func.CleanLogs)
			log.GET("/recent", log_func.GetRecentLogCount)
			log.GET("/errors", log_func.GetErrorLogs)
		}

		r.NoRoute(func(c *gin.Context) {
			c.String(http.StatusOK, "no route found")
		})
	}

	if conf.ViewPath == "" {
		conf.ViewPath = "./view"
	}
	r.StaticFS("/admin", http.Dir(conf.ViewPath))
	r.StaticFile("/favicon.ico", conf.ViewPath+"/favicon.ico")
}
