from drf_spectacular.openapi import AutoSchema
from drf_spectacular.extensions import OpenApiSerializerFieldExtension
from drf_spectacular.types import OpenApiTypes
from drf_spectacular.plumbing import build_basic_type


class EmptyTagAuthoSchema(AutoSchema):
    def get_tags(self):
        return ['']


class FixDatetimeOpenApi(OpenApiSerializerFieldExtension):
    target_class = 'rest_framework.serializers.DateTimeField'

    def map_serializer_field(self, auto_schema, direction):
        return build_basic_type(OpenApiTypes.INT)


def preprocessing_filter_spec(endpoints):
    filtered = []
    for (path, path_regex, method, callback) in endpoints:
        # Remove all but DRF API endpoints
        if path.startswith("/api/"):
            filtered.append((path, path_regex, method, callback))
    return filtered
