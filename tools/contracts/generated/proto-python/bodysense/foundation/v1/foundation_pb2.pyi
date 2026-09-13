from buf.validate import validate_pb2 as _validate_pb2
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from typing import ClassVar as _ClassVar, Optional as _Optional

DESCRIPTOR: _descriptor.FileDescriptor

class FoundationCommand(_message.Message):
    __slots__ = ("name", "expected_revision", "note")
    NAME_FIELD_NUMBER: _ClassVar[int]
    EXPECTED_REVISION_FIELD_NUMBER: _ClassVar[int]
    NOTE_FIELD_NUMBER: _ClassVar[int]
    name: str
    expected_revision: int
    note: str
    def __init__(self, name: _Optional[str] = ..., expected_revision: _Optional[int] = ..., note: _Optional[str] = ...) -> None: ...
