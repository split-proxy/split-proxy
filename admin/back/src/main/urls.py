from django.contrib import admin
from django.urls import path, include
from django.conf import settings
from django.contrib.staticfiles.urls import staticfiles_urlpatterns
from drf_spectacular.views import SpectacularSwaggerView, SpectacularRedocView, SpectacularAPIView

urlpatterns = [
    path('api/v1/', include(('core.urls', 'api'))),
]

if settings.DEBUG:
    urlpatterns += [
        path('admin/', admin.site.urls),
    ]

if settings.SHOW_SWAGGER:
    urlpatterns += [
        path('schema/', SpectacularAPIView.as_view(api_version='v1'), name='schema'),
        path('swagger/', SpectacularSwaggerView.as_view(url_name='schema'), name='swagger-ui'),
        path('redoc/', SpectacularRedocView.as_view(url_name='schema'), name='redoc'),
    ]

urlpatterns += staticfiles_urlpatterns()
