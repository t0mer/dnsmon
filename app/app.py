from server import Server
from utils import Utils
from sqliteconnector import SqliteConnector









if __name__ == "__main__":
    server = Server()
    connector = SqliteConnector()
    server.start()