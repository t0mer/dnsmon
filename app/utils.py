import httpx
import socket
import requests
from loguru import logger

class QueryTimeoutException(Exception):
    """A query timeout exception."""

    pass


class QueryException(Exception):
    """A query exception."""

    pass

class InvalidTypeException(Exception):
    """An invalid query type exception. """

class InvalidServerException(Exception):
    """An invalid server exception."""

    pass


class Utils():
    def __init__(self):
        self.wmd_base_url = "https://www.whatsmydns.net/api/details?server={}&type={}&query={}"
        self.RESULT_EMOJIS = {"succeeded": "✅", "failed": "❌", "timeout": "⌛", "unknown": "❓"}
        self.headers = {"user-agent": f"dnsmon/{'1.0.0'}"}
        self.types=['A','AAAA','CNAME','MX','NS','PTR','SOA','SRV','TXT','CAA']

    
    def check_record(self,server,type,query):
        try:
            if type not in self.types:
                raise InvalidTypeException

            self.url = self.wmd_base_url.format(server,type,query)
            response = requests.get(url=self.url,headers=self.headers)

            if response.status_code !=200:
                if response.json()["errors"]["server"][0] == "Invalid server":
                    raise InvalidServerException(response.json())
                else:
                    raise QueryException(response.json())

            if response.json()["data"][0]["response"] == "DNS query timed out":
                raise QueryTimeoutException(response["data"][0]["response"])
            
            data = response.json()["data"][0]
            if data["rcode"] == "NOERROR":
                result = "succeeded"
            elif data["rcode"] == "SERVFAIL":
                result = "failed"
            else:
                result = "unknown"

            if type=='SOA':
                answer = data['response'][0].split(' ', 2)
                answer = ' '.join(answer)
            else:
                answer = ", ".join(answer.split()[-1] for answer in data["answers"])


            return answer


        except QueryTimeoutException:
            result = "timeout"
            answer = "DNS query timed out."
            return answer, result

        except InvalidServerException:
            result = "failed"
            answer = "Invalid server."
            return answer, result

        except InvalidTypeException:
            result = "failed"
            answer = "Invalid qery type"
            return answer, result
        
        except Exception as e:
            logger.error(str(e))
            return "Aw, Snap!"

        