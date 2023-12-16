import sqlite3
from sqlite3 import Error
from loguru import logger

class SqliteConnector:
    def __init__(self):
        self.db_file = "db/dnsmon.db"
        self.conn = None

    def open_connection(self):
        try:
            self.conn = sqlite3.connect(self.db_file)
        except Error as e:
            logger.error(str(e))

    def close_connection(self):
        try:
            self.conn.close()
        except Error as e:
            logger.error(str(e))

    def create_tables(self):
        logger.info('#')
        self.open_connection()
        create_servers_table = """ CREATE TABLE IF NOT EXISTS servers (
                                    id text PRIMARY KEY,
                                    latitude integer NOT NULL,
                                    longitude integer NOT NULL,
                                    location text NOT NULL,
                                    provider text NOT NULL,
                                    country text NOT NULL,
                                    status integer NOT NULL
                                ); """

        create_monitored_records_table = """ CREATE TABLE IF NOT EXISTS monitored_records (
                            DomainId integer PRIMARY KEY,
                            Record text NOT NULL,
                            RecordType text NOT NULL,
                            ExpectedValue text NOT NULL,
                            Active integer DEFAULT 1 NOT NULL
                        ); """
        try:
            c = self.conn.cursor()
            c.execute(create_servers_table) 
            c.execute(create_monitored_records_table)
            c.close()
            self.conn.close()          
        except Error as e:
            logger.error(str(e))
    def add_server(self,id,latitude,longitude,location,provider,country,status):
        try:
            if not self.is_server_exists(id):
                Tunnel = (id,latitude,longitude,location,provider,country,status,)
                self.open_connection()
                sql =  """ INSERT INTO servers(id,latitude,longitude,location,provider,country,status) VALUES (?,?,?,?,?,?,?)"""
                cur = self.conn.cursor()
                cur.execute(sql,Tunnel)
                self.conn.commit()
                self.conn.close()
                return str(cur.lastrowid>0), "Server addedd successfully"
        except Error as e:
            logger.error(str(e))
            return False, str(e)

    def update_server_status(self, id, status):
        try:
            self.open_connection()
            sql = ''' UPDATE monitored_tunnels
              SET Status = ? 
              WHERE id = ?'''
            cur = self.conn.cursor()
            cur.execute(sql,(status,id))
            self.conn.commit()
            cur.close()
            self.conn.close()
            return True
        except Error as e:
            logger.error(str(e))
            return False

    def is_server_exists(self, id):
        try:
            self.open_connection()
            cursor = self.conn.cursor()
            query = "SELECT * FROM servers where id = '" + id + "'"
            cursor.execute(query)
            rows = [dict((cursor.description[i][0], value) \
               for i, value in enumerate(row)) for row in cursor.fetchall()]
            return (True if rows else False)
        except Exception as e:
            self.conn.close()
            logger.error(e)
            return False

    def get_server_by_id(self, id):
        try:
            self.open_connection()
            cursor = self.conn.cursor()
            query = "SELECT * FROM servers where id = '" + id + "'"
            cursor.execute(query)
            rows = cursor.fetchall()
            return rows
        except Exception as e:
            self.conn.close()
            logger.error(e)
            return None

    def get_active_servers(self,api_call=False):
        try:
            self.open_connection()
            cursor = self.conn.cursor()
            query = "SELECT * FROM servers where Status = 1"
            cursor.execute(query)
            if api_call == True:
                rows = [dict((cursor.description[i][0], value) \
                for i, value in enumerate(row)) for row in cursor.fetchall()]
                cursor.close()
                return (rows[0] if rows else None) if False else rows
            else:
                rows = cursor.fetchall()
            return rows
        except Exception as e:
            self.conn.close()
            logger.error(e)
            return None

    def get_monitored_records(self, api_call=False):
        monitored_domain_list = []
        logger.debug("api_call = " + str(api_call))
        try:
            self.open_connection()
            cursor = self.conn.cursor()
            query = "SELECT DomainId,Record,RecordType,ExpectedValue,Active FROM monitored_records"
            cursor.execute(query)
            if api_call == True:
                rows = [dict((cursor.description[i][0], value) \
                for i, value in enumerate(row)) for row in cursor.fetchall()]
                cursor.close()
                return (rows[0] if rows else None) if False else rows
            else:
                rows = cursor.fetchall()
            return rows
        except Error as e:
            logger.error(str(e))
            return monitored_domain_list
        finally:
            self.close_connection()

    def add_monitored_record(self,Record,RecordType,ExpectedValue,Active):
        try:
            if not self.record_is_monitored(Record,RecordType):
                record = (Record,RecordType,ExpectedValue,Active,)
                self.open_connection()
                sql =  """ INSERT INTO monitored_records(Record,RecordType,ExpectedValue,Active) VALUES (?,?,?,?)"""
                cur = self.conn.cursor()
                cur.execute(sql,record)
                self.conn.commit()
                self.conn.close()
                logger.info("Record addedd successfully")
                return str(cur.lastrowid>0), "Record addedd successfully"
            else:
                logger.info("The specific record is already being monitored")
        except Error as e:
            logger.error(str(e))
            return False, str(e)

    def record_is_monitored(self, Record,RecordType):
        try:
            self.open_connection()
            cursor = self.conn.cursor()
            query = "SELECT * FROM monitored_records where Record = '" + Record + "' and RecordType = '" + RecordType +"'"
            cursor.execute(query)
            rows = [dict((cursor.description[i][0], value) \
               for i, value in enumerate(row)) for row in cursor.fetchall()]
            return (True if rows else False)
        except Exception as e:
            self.conn.close()
            logger.error(e)
            return False



if __name__ == "__main__":
    con = SqliteConnector()
    con.create_tables()
