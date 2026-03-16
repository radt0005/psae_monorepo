from pydantic import BaseModel



class File(BaseModel):
    path: str


class Directory(BaseModel):
    path: str


