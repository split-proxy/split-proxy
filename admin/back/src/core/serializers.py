import ipaddress
from rest_framework import serializers

from django.contrib.auth import get_user_model
from core import models, fields, services
from django.conf import settings

User = get_user_model()


class TimeStampMixin(serializers.Serializer):
    updated_at = fields.TimestampField(required=False, read_only=True)
    created_at = fields.TimestampField(required=False, read_only=True)


class CreateUpdatePasswordMixin:
    def create(self, validated_data):
        password = validated_data.pop('password')
        model = self.Meta.model

        user = model(**validated_data)
        user.set_password(password)
        user.save()

        return user

    def update(self, instance, validated_data):
        password = validated_data.pop('password', None)

        for attr, value in validated_data.items():
            setattr(instance, attr, value)

        if password:
            instance.set_password(password)

        instance.save()
        return instance


class CurrentProfileSerializer(serializers.ModelSerializer):

    class Meta:
        model = User
        fields = ('id', 'username')


class ProfileSerializer(TimeStampMixin, CreateUpdatePasswordMixin, serializers.ModelSerializer):
    password = serializers.CharField(
        write_only=True,
        required=True,
        min_length=8,
    )

    class Meta:
        model = User
        fields = ('id', 'username', 'created_at', 'password', 'is_superuser',)
        read_only_fields = ('id', 'created_at', 'is_superuser')


class ProxySerializer(TimeStampMixin, CreateUpdatePasswordMixin, serializers.ModelSerializer):
    password = serializers.CharField(
        write_only=True,
        required=True,
        min_length=8,
    )

    class Meta:
        model = models.Proxy
        fields = ('id', 'username', 'password', 'created_at', 'updated_at')


class WorkerGroupSerializer(TimeStampMixin, serializers.ModelSerializer):
    workers = serializers.IntegerField(source='workers_count', read_only=True)
    online = serializers.IntegerField(source='online_count', read_only=True)
    offline = serializers.IntegerField(source='offline_count', read_only=True)

    def validate_group_name(self, value):
        if value == settings.DEFAULT_GROUP_NAME:
            raise serializers.ValidationError("Cant't create default group")
        return value

    class Meta:
        model = models.WorkerGroup
        fields = (
            'id', 'group_name', 'workers', 'online', 'offline', 'created_at', 'updated_at',
        )


class WorkerSerializer(TimeStampMixin, serializers.ModelSerializer):

    class Meta:
        model = models.Worker
        fields = ('guid', 'ip_address', 'group', 'created_at', 'updated_at', 'online')
        read_only_fields = ('guid', 'ip_address', 'created_at', 'updated_at', 'online')


class DomainRouteSerializer(serializers.Serializer):
    pattern = serializers.CharField(max_length=255)
    worker_group = serializers.PrimaryKeyRelatedField(
        queryset=models.WorkerGroup.objects.all()
    )


class DomainsRouteSerializer(serializers.Serializer):
    patterns = serializers.ListField(
        child=serializers.CharField(max_length=255),
        allow_empty=False,
    )
    worker_group = serializers.PrimaryKeyRelatedField(
        queryset=models.WorkerGroup.objects.all()
    )

    def validate_patterns(self, value):
        for pattern in value:
            if services.domain_route_exists(pattern):
                raise serializers.ValidationError("Such pattern already exist: %s" % pattern)
        return value


class CIDRRouteSerializer(serializers.Serializer):
    cidr = serializers.CharField(max_length=64)
    worker_group = serializers.PrimaryKeyRelatedField(
        queryset=models.WorkerGroup.objects.all()
    )

    def validate_cidr(self, value):
        try:
            network = ipaddress.ip_network(value, strict=False)
        except ValueError:
            raise serializers.ValidationError("Invalid CIDR")
        return str(network)


class CIDRsRouteSerializer(serializers.Serializer):
    cidrs = serializers.ListField(
        child=serializers.CharField(max_length=255),
        allow_empty=False,
    )
    worker_group = serializers.PrimaryKeyRelatedField(
        queryset=models.WorkerGroup.objects.all()
    )

    def validate_cidrs(self, value):
        for cidr in value:
            try:
                ipaddress.ip_network(cidr, strict=False)
            except ValueError:
                raise serializers.ValidationError("Invalid CIDRs")
            if services.cidr_route_exists(cidr):
                raise serializers.ValidationError("Such cidr already exist: %s" % cidr)
        return value


class PaginatedCIDRRouteSerializer(serializers.Serializer):
    count = serializers.IntegerField()
    next = serializers.CharField(allow_null=True)
    previous = serializers.CharField(allow_null=True)
    results = CIDRRouteSerializer(many=True)


class PaginatedDomainRouteSerializer(serializers.Serializer):
    count = serializers.IntegerField()
    next = serializers.CharField(allow_null=True)
    previous = serializers.CharField(allow_null=True)
    results = DomainRouteSerializer(many=True)


class DefaultRouteSerializer(serializers.Serializer):
    worker_group = serializers.IntegerField(min_value=0)
