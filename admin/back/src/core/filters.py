import django_filters

from core import models


class WorkerFilterSet(django_filters.FilterSet):
    guid = django_filters.CharFilter(
        field_name="guid",
        lookup_expr="icontains",
    )

    group = django_filters.CharFilter(
        method="filter_group",
    )

    def filter_group(self, queryset, name, value):
        if value == "none":
            return queryset.filter(group__isnull=True)

        if value.isdigit():
            return queryset.filter(group_id=int(value))

        return queryset.none()

    class Meta:
        model = models.Worker
        fields = ["guid", "group"]
