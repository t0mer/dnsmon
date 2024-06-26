import uvicorn
import requests
from utils import Utils
from loguru import logger
from sqliteconnector import SqliteConnector
from fastapi import FastAPI, Request, File, Form, UploadFile
from fastapi.responses import UJSONResponse
from fastapi.templating import Jinja2Templates
from fastapi.responses import HTMLResponse
from fastapi.staticfiles import StaticFiles
from fastapi.responses import JSONResponse
from starlette.responses import FileResponse
from starlette.responses import StreamingResponse
from fastapi.middleware.cors import CORSMiddleware
from fastapi.encoders import jsonable_encoder
from starlette_exporter import PrometheusMiddleware, handle_metrics


class Server:
    def __init__(self):
        self.connector = SqliteConnector()
        self.connector.create_tables()
        self.servers_url = "https://wmd.techblog.co.il/servers"
        self.utils = Utils(self.connector)
        self.tags_metadata = [
            {
                "name": "servers",
                "description": "Get servers list",
            },
            {
                "name": "records",
                "description": "Get list of record types",
            },
        
        ]

        self.app = FastAPI(title="DnsMon", description="DNS records propogation checker and change detection", version='1.0.0', openapi_tags=self.tags_metadata, contact={"name": "Tomer Klein", "email": "tomer.klein@gmail.com", "url": "https://github.com/t0mer/wmd-servers-scrapper"})
        self.app.mount("/dist", StaticFiles(directory="dist"), name="dist")
        self.app.mount("/plugins", StaticFiles(directory="plugins"), name="plugins")
        self.app.mount("/js", StaticFiles(directory="dist/js"), name="js")
        self.app.mount("/css", StaticFiles(directory="dist/css"), name="css")
        self.templates = Jinja2Templates(directory="templates/")
        self.app.add_middleware(PrometheusMiddleware)
        self.app.add_route("/metrics", handle_metrics)
        self.origins = ["*"]

        self.app.add_middleware(
            CORSMiddleware,
            allow_origins=self.origins,
            allow_credentials=True,
            allow_methods=["*"],
            allow_headers=["*"],
        )


        @self.app.get("/servers")
        def home(request: Request):
            """
            Homepage
            """
            return self.templates.TemplateResponse('servers.html', context={'request': request })

        @self.app.get("/")
        def home(request: Request):
            """
            Whatsmydns page
            """
            logger.info("Loading default page")
            return self.templates.TemplateResponse('index.html', context={'request': request })

        @self.app.get("/api/servers",tags=['servers'], summary="Get list of servers")
        def get_servers(request: Request):
            """
            Return the list of WhatsMyDNS servers
            """
            try:
                return JSONResponse(self.connector.get_active_servers(True))
            except Exception as e:
                logger.error("Error fetch images, " + str(e))
                return None

        @self.app.get("/api/types",tags=['records'], summary="Get list of record types")
        def get_types(request: Request):
            """
            Retuens the list of record types (A,AAAA,MX, etc.)
            """
            try:
                return JSONResponse(self.utils.types)
            except Exception as e:
                logger.error("Error fetch images, " + str(e))
                return None
            
        @self.app.get("/api/monitored",tags=['records'], summary="Get list of monitored records")
        def get_monitored(request: Request):
            """
            Returns the list of monitored records
            """
            try:
                monitored_records = self.connector.get_monitored_records(True)
                return JSONResponse(monitored_records)
            except Exception as e:
                logger.error("Error fetch images, " + str(e))
                return None
             
        @self.app.get("/api/query",tags=['records'], summary="Get list of monitored records")
        def run_query(request: Request,server:str,type:str,query:str):
            """
            Run the requested query to validate
            """
            try:
                response = self.utils.check_record(server,type,query)
                return JSONResponse(response)
            except Exception as e:
                logger.error("Error fetch images, " + str(e))
                return None
            
        @self.app.get("/api/servers/update",tags=['servers'], summary="Update servers list")
        def update_servers(request: Request):
            """
            Update the Whatsmydns servers list
            """
            try:
                
                response = self.utils.update_servers_list()
                response = {"success":True,"messaege":""}
                return JSONResponse(response)
            except Exception as e:
                logger.error("Error fetch images, " + str(e))
                response = {"success":True,"message":str(e)}
                return JSONResponse(response)


    def start(self):
        uvicorn.run(self.app, host="0.0.0.0", port=8082)


