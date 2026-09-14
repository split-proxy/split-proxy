from django.db.models import Count, Q
from django.conf import settings
from rest_framework import generics
from rest_framework.authtoken.views import ObtainAuthToken
from rest_framework.parsers import JSONParser
from rest_framework.permissions import IsAuthenticated
from rest_framework.views import APIView
from rest_framework import status, serializers as drf_serializers
from rest_framework.response import Response
from drf_spectacular.utils import (
    extend_schema,
    OpenApiExample,
    OpenApiParameter,
    OpenApiResponse,
    inline_serializer
)

from core.pagination import CustomPageNumberPagination
from core import serializers, models, permissions, services


@extend_schema(tags=['Auth methods'])
class ObtainTokenView(ObtainAuthToken):
    parser_classes = [JSONParser]


@extend_schema(tags=['Profile methods'])
class ProfileAPIView(APIView):

    def get(self, request):
        serializer = serializers.CurrentProfileSerializer(instance=self.request.user)
        return Response(serializer.data, status=status.HTTP_200_OK)


@extend_schema(tags=['Profile methods'])
class ProfileListCreateSerializerAPIView(generics.ListCreateAPIView):
    queryset = models.Profile.objects.all()
    serializer_class = serializers.ProfileSerializer


@extend_schema(tags=['Profile methods'])
class ProfileRetrieveUpdateDestroyAPIView(generics.RetrieveUpdateDestroyAPIView):
    queryset = models.Profile.objects.all()
    serializer_class = serializers.ProfileSerializer
    http_method_names = ("get", "patch", "delete")
    permission_classes = (
        IsAuthenticated,
        permissions.CanDeleteProfile,
    )


@extend_schema(tags=['Proxy methods'])
class ProxyListCreateSerializerAPIView(generics.ListCreateAPIView):
    queryset = models.Proxy.objects.all()
    serializer_class = serializers.ProxySerializer


@extend_schema(tags=['Proxy methods'])
class ProxyRetrieveUpdateDestroyAPIView(generics.RetrieveUpdateDestroyAPIView):
    queryset = models.Proxy.objects.all()
    serializer_class = serializers.ProxySerializer
    http_method_names = ("get", "patch", "delete")


@extend_schema(tags=['Worker methods'])
class WorkerGroupListCreateSerializerAPIView(generics.ListCreateAPIView):
    queryset = models.WorkerGroup.objects.annotate(
        workers_count=Count('workers', distinct=True),
        online_count=Count(
            'workers',
            filter=Q(workers__online=True),
            distinct=True,
        ),
        offline_count=Count(
            'workers',
            filter=Q(workers__online=False),
            distinct=True,
        ),
    )
    serializer_class = serializers.WorkerGroupSerializer


@extend_schema(tags=['Worker methods'])
class WorkerGroupRetrieveUpdateDestroyAPIView(generics.RetrieveUpdateDestroyAPIView):
    queryset = models.WorkerGroup.objects.annotate(
        workers_count=Count('workers', distinct=True),
        online_count=Count(
            'workers',
            filter=Q(workers__online=True),
            distinct=True,
        ),
        offline_count=Count(
            'workers',
            filter=Q(workers__online=False),
            distinct=True,
        ),
    )
    serializer_class = serializers.WorkerGroupSerializer
    http_method_names = ("get", "patch", "delete")


@extend_schema(tags=['Worker methods'])
class WorkerListSerializerAPIView(generics.ListAPIView):
    queryset = models.Worker.objects.all()
    serializer_class = serializers.WorkerSerializer


@extend_schema(tags=['Worker methods'])
class WorkerRetrieveUpdateDestroyAPIView(generics.RetrieveUpdateDestroyAPIView):
    queryset = models.Worker.objects.all()
    serializer_class = serializers.WorkerSerializer
    http_method_names = ("get", "patch", "delete")


@extend_schema(tags=["Routing methods"])
class DomainRouteListCreateAPIView(APIView):

    @extend_schema(
        summary="List domain routes",
        parameters=[
            OpenApiParameter(
                name="search",
                type=str,
                location=OpenApiParameter.QUERY,
                required=False,
                description="Search by domain pattern",
            ),
            OpenApiParameter(
                name="page",
                type=int,
                location=OpenApiParameter.QUERY,
                required=False,
                description="Page number",
            ),
            OpenApiParameter(
                name="page_size",
                type=int,
                location=OpenApiParameter.QUERY,
                required=False,
                description="Number of items per page",
            ),
        ],
        responses=serializers.PaginatedDomainRouteSerializer,
    )
    def get(self, request):
        search = request.query_params.get("search")
        routes = services.get_domain_routes(search=search)
        paginator = CustomPageNumberPagination()
        page = paginator.paginate_queryset(routes, request)
        return paginator.get_paginated_response(page)

    @extend_schema(
        summary="Create domain route",
        request=serializers.DomainRouteSerializer,
        responses={
            201: serializers.DomainsRouteSerializer,
            409: OpenApiResponse(
                description="Route already exists.",
            ),
        },
    )
    def post(self, request):
        serializer = serializers.DomainsRouteSerializer(data=request.data)
        serializer.is_valid(raise_exception=True)
        return Response(
            services.create_domain_route(**serializer.validated_data),
            status=status.HTTP_201_CREATED
        )


@extend_schema(tags=["Routing methods"])
class DomainRouteDetailAPIView(APIView):

    @extend_schema(
        summary="Get domain route",
        parameters=[
            OpenApiParameter(
                name="pattern",
                type=str,
                location=OpenApiParameter.PATH,
                description="Domain route pattern.",
                examples=[
                    OpenApiExample(
                        "Exact domain",
                        value="example.com",
                    ),
                    OpenApiExample(
                        "Wildcard domain",
                        value="*.example.com",
                    ),
                ],
            ),
        ],
        responses={
            200: serializers.DomainRouteSerializer,
            404: OpenApiResponse(
                description="Route not found.",
            ),
        },
    )
    def get(self, request, pattern):
        route = services.get_domain_route(pattern)

        if route is None:
            return Response(
                {"detail": "Route not found."},
                status=status.HTTP_404_NOT_FOUND,
            )

        return Response(route)

    @extend_schema(
        summary="Update domain route",
        request=inline_serializer(
            name="DomainRouteUpdate",
            fields={
                "worker_group": drf_serializers.PrimaryKeyRelatedField(
                    queryset=models.WorkerGroup.objects.all(),
                ),
            },
        ),
        responses={
            200: serializers.DomainRouteSerializer,
            404: OpenApiResponse(
                description="Route not found.",
            ),
        },
    )
    def patch(self, request, pattern):
        request.data.update({'pattern': pattern})
        serializer = serializers.DomainRouteSerializer(
            data=request.data,
        )
        serializer.is_valid(raise_exception=True)
        return Response(services.update_domain_route(**serializer.validated_data))

    @extend_schema(
        summary="Delete domain route",
        responses={
            204: OpenApiResponse(
                description="Route deleted.",
            ),
            404: OpenApiResponse(
                description="Route not found.",
            ),
        },
    )
    def delete(self, request, pattern):
        deleted = services.delete_domain_route(pattern)

        if not deleted:
            return Response({"detail": "Route not found."}, status=status.HTTP_404_NOT_FOUND)

        return Response(status=status.HTTP_204_NO_CONTENT)


@extend_schema(tags=["Routing methods"])
class CIDRRouteListCreateAPIView(APIView):

    @extend_schema(
        summary="List CIDR routes",
        parameters=[
            OpenApiParameter(
                name="search",
                type=str,
                location=OpenApiParameter.QUERY,
                required=False,
                description="Search by CIDR",
            ),
            OpenApiParameter(
                name="page",
                type=int,
                location=OpenApiParameter.QUERY,
                required=False,
                description="Page number",
            ),
            OpenApiParameter(
                name="page_size",
                type=int,
                location=OpenApiParameter.QUERY,
                required=False,
                description="Number of items per page",
            ),
        ],
        responses=serializers.PaginatedCIDRRouteSerializer,
    )
    def get(self, request):
        search = request.query_params.get("search")

        routes = services.get_cidr_routes(search=search)

        paginator = CustomPageNumberPagination()
        page = paginator.paginate_queryset(routes, request)

        return paginator.get_paginated_response(page)

    @extend_schema(
        summary="Create CIDR route",
        request=serializers.CIDRsRouteSerializer,
        responses={
            201: serializers.CIDRRouteSerializer,
            409: OpenApiResponse(
                description="Route already exists.",
            ),
        },
    )
    def post(self, request):
        serializer = serializers.CIDRsRouteSerializer(data=request.data)
        serializer.is_valid(raise_exception=True)
        return Response(
            services.create_cidr_route(**serializer.validated_data),
            status=status.HTTP_201_CREATED
        )


@extend_schema(tags=["Routing methods"])
class CIDRRouteDetailAPIView(APIView):

    @extend_schema(
        summary="Get CIDR route",
        parameters=[
            OpenApiParameter(
                name="cidr",
                type=str,
                location=OpenApiParameter.PATH,
                description="CIDR network.",
                examples=[
                    OpenApiExample(
                        "IPv4",
                        value="192.168.1.0/24",
                    ),
                    OpenApiExample(
                        "IPv6",
                        value="2001:db8::/32",
                    ),
                ],
            ),
        ],
        responses={
            200: serializers.CIDRRouteSerializer,
            404: OpenApiResponse(
                description="Route not found.",
            ),
        },
    )
    def get(self, request, cidr):
        route = services.get_cidr_route(cidr)

        if route is None:
            return Response(
                {"detail": "Route not found."},
                status=status.HTTP_404_NOT_FOUND,
            )

        return Response(route)

    @extend_schema(
        summary="Update CIDR route",
        request=inline_serializer(
            name="CidrRouteUpdate",
            fields={
                "worker_group": drf_serializers.PrimaryKeyRelatedField(
                    queryset=models.WorkerGroup.objects.all(),
                ),
            },
        ),
        responses={
            200: serializers.CIDRRouteSerializer,
            404: OpenApiResponse(
                description="Route not found.",
            ),
        },
    )
    def patch(self, request, cidr):
        data = {
            **request.data,
            "cidr": cidr,
        }

        serializer = serializers.CIDRRouteSerializer(data=data)
        serializer.is_valid(raise_exception=True)

        if not services.cidr_route_exists(cidr):
            return Response(
                {"detail": "Route not found."},
                status=status.HTTP_404_NOT_FOUND,
            )

        return Response(services.update_cidr_route(**serializer.validated_data))

    @extend_schema(
        summary="Delete CIDR route",
        responses={
            204: OpenApiResponse(
                description="Route deleted.",
            ),
            404: OpenApiResponse(
                description="Route not found.",
            ),
        },
    )
    def delete(self, request, cidr):
        if not services.delete_cidr_route(cidr):
            return Response(
                {"detail": "Route not found."},
                status=status.HTTP_404_NOT_FOUND,
            )

        return Response(status=status.HTTP_204_NO_CONTENT)


@extend_schema(tags=["Routing methods"])
class DefaultRouteAPIView(APIView):

    @extend_schema(
        summary="Get default route",
        responses={
            200: serializers.DefaultRouteSerializer,
            404: OpenApiResponse(
                description="Default route is not configured.",
            ),
        },
    )
    def get(self, request):
        route = services.get_default_route()

        if route is None:
            return Response(
                {"detail": "Default route is not configured."},
                status=status.HTTP_404_NOT_FOUND,
            )

        return Response(route)

    @extend_schema(
        summary="Update default route",
        request=serializers.DefaultRouteSerializer,
        responses={
            200: serializers.DefaultRouteSerializer,
        },
    )
    def patch(self, request):
        serializer = serializers.DefaultRouteSerializer(data=request.data)
        serializer.is_valid(raise_exception=True)
        return Response(services.update_default_route(**serializer.validated_data))


class ConfigAPIView(APIView):
    authentication_classes = ()
    permission_classes = ()

    def get(self, request):
        return Response({
            "default_group_name": settings.DEFAULT_GROUP_NAME,
        })
