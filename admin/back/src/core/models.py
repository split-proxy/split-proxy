import uuid

from django.contrib.auth.models import AbstractUser
from django.utils.translation import gettext_lazy as _
from django.db import models
from django.contrib.auth.hashers import make_password


class AbstractTimeStampModel(models.Model):
    created_at = models.DateTimeField(auto_now_add=True)
    updated_at = models.DateTimeField(auto_now=True)

    class Meta:
        abstract = True


class Profile(AbstractTimeStampModel, AbstractUser):
    email = models.EmailField(blank=True, null=True)
    REQUIRED_FIELDS = []

    class Meta:
        ordering = ('id',)


class Proxy(AbstractTimeStampModel):
    username = models.CharField(verbose_name=_('Proxy username'), max_length=255, unique=True)
    password = models.CharField(verbose_name=_('Proxy password'), max_length=255)

    class Meta:
        verbose_name = _('Proxy')
        verbose_name_plural = _('Proxies')
        ordering = ('id',)

    def __str__(self):
        return 'Proxy: %s' % self.username

    def set_password(self, raw_password):
        self.password = make_password(raw_password)


class WorkerGroup(AbstractTimeStampModel):
    group_name = models.CharField(verbose_name=_('Group name'), max_length=255, blank=True, unique=True)

    class Meta:
        ordering = ('id',)
        verbose_name = _('Worker group')
        verbose_name_plural = _('Worker groups')

    def __str__(self):
        return 'WorkerGroup: %s' % self.group_name


class Worker(AbstractTimeStampModel):
    guid = models.UUIDField(primary_key=True, default=uuid.uuid4, editable=False)
    ip_address = models.GenericIPAddressField(null=True, blank=True)
    group = models.ForeignKey(
        WorkerGroup,
        on_delete=models.SET_NULL,
        null=True,
        blank=True,
        related_name="workers",
    )
    online = models.BooleanField(verbose_name=_('Online/offline'))

    class Meta:
        verbose_name = _('Worker')
        verbose_name_plural = _('Workers')
        ordering = ('-created_at',)

    def __str__(self):
        return 'Worker: %s' % self.guid
