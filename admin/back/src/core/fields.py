import datetime
from rest_framework import serializers
from drf_spectacular.utils import extend_schema_field


@extend_schema_field({
    'type': 'int',
    'format': 'timestamp',
    'example': 1751021803,
    'description': 'Datetime in timestamp format'

})
class TimestampField(serializers.Field):

    def to_representation(self, value):
        return value and int(value.timestamp())

    def to_internal_value(self, value):
        try:
            return datetime.datetime.fromtimestamp(int(value))
        except (TypeError, ValueError):
            raise serializers.ValidationError("Invalid Unix timestamp")
