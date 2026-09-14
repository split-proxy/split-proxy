from django.urls import path
from core import views

urlpatterns = [
    path('get_token', views.ObtainTokenView.as_view(), name="get_token"),
    path('current_profile', views.ProfileAPIView.as_view(), name='current_profile'),
    path('profile', views.ProfileListCreateSerializerAPIView.as_view(), name='profile'),
    path('profile/<int:pk>', views.ProfileRetrieveUpdateDestroyAPIView.as_view(), name='profile'),

    path('proxy', views.ProxyListCreateSerializerAPIView.as_view(), name='proxy'),
    path('proxy/<int:pk>', views.ProxyRetrieveUpdateDestroyAPIView.as_view(), name='proxy'),

    path('worker_group', views.WorkerGroupListCreateSerializerAPIView.as_view(), name='worker_group'),
    path('worker_group/<int:pk>', views.WorkerGroupRetrieveUpdateDestroyAPIView.as_view(), name='worker_group'),

    path('worker', views.WorkerListSerializerAPIView.as_view(), name='worker'),
    path('worker/<uuid:pk>', views.WorkerRetrieveUpdateDestroyAPIView.as_view(), name='worker'),

    path("routing/domains", views.DomainRouteListCreateAPIView.as_view(), name="domain"),
    path("routing/domains/<path:pattern>", views.DomainRouteDetailAPIView.as_view(), name="domain"),
    path("routing/cidr", views.CIDRRouteListCreateAPIView.as_view(), name="cidr"),
    path("routing/cidr/<path:cidr>", views.CIDRRouteDetailAPIView.as_view(), name="cidr"),
    path("routing/default", views.DefaultRouteAPIView.as_view(), name="default-route"),

    path("config", views.ConfigAPIView.as_view(), name="config"),
]
