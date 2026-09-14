from django.contrib import admin
from django.contrib.auth.admin import UserAdmin
from django.utils.translation import gettext_lazy as _
from core import models


@admin.register(models.Profile)
class ProfilerAdmin(UserAdmin):
    fieldsets = (
        (None, {'fields': ('username', 'password')}),
        (_('Personal info'), {'fields': (
            'first_name', 'last_name', 'email',
        )}),
        (_('Permissions'), {'fields': (
            'is_active', 'is_staff', 'is_superuser',
            'groups', 'user_permissions')
        }),
        (_('Important dates'), {'fields': ('last_login', 'date_joined')}),
    )


@admin.register(models.Proxy)
class ProxyAdmin(admin.ModelAdmin):
    pass


@admin.register(models.WorkerGroup)
class WorkerGroupAdmin(admin.ModelAdmin):
    pass


@admin.register(models.Worker)
class WorkerWorkerAdmin(admin.ModelAdmin):
    pass
