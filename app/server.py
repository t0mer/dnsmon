import uvicorn
import requests
from loguru import logger
from fastapi.responses import JSONResponse
from sqliteconnector import SqliteConnector
from fastapi import FastAPI, Request

class Server:
    def __init__(self):
        self.connector = SqliteConnector()
        self.connector.create_tables()
        self.servers_url = "https://wmd.techblog.co.il/servers"

        self.tags_metadata = [
            {
                "name": "servers",
                "description": "Get servers list",
            },
        ]

        self.app = FastAPI(title="DnsMon", description="DNS records propogation checker and change detection", version='1.0.0', openapi_tags=self.tags_metadata, contact={"name": "Tomer Klein", "email": "tomer.klein@gmail.com", "url": "https://github.com/t0mer/wmd-servers-scrapper"})

        @self.app.get("/api/servers",tags=['servers'], summary="Get list of servers")
        def get_servers(request: Request):
            try:
                servers = requests.get(self.servers_url).json()
                for server in servers:
                    self.connector.add_server(server['id'], server['latitude'],server['longitude'],server['location'],server['provider']
                                              ,server['country'],server['status'])
                return JSONResponse(self.connector.get_active_servers(True))
            except Exception as e:
                logger.error("Error fetch images, " + str(e))
                return None

    def start(self):
        uvicorn.run(self.app, host="0.0.0.0", port=8082)

