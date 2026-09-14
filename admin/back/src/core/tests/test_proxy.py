import pytest

from django.urls import reverse
from rest_framework import status
from core import models
from ddf import G


@pytest.mark.django_db(transaction=True)
def test_get_proxies(client, authorization_header):
    G(models.Proxy)
    resp = client.get(reverse('api:proxy'), headers=authorization_header)
    assert resp.status_code == status.HTTP_200_OK
    assert resp.json()['count'] == models.Proxy.objects.count()


@pytest.mark.django_db(transaction=True)
def test_create_proxy(client, authorization_header):
    USERNAME = 'test_proxy'
    resp = client.post(
        reverse('api:proxy'),
        data={
            'username': USERNAME,
            'password': 'test_password',
        },
        headers=authorization_header
    )
    assert resp.status_code == status.HTTP_201_CREATED
    assert models.Proxy.objects.filter(username=USERNAME).count() == 1


@pytest.mark.django_db(transaction=True)
def test_update_proxy(client, authorization_header):
    PASSWORD = 'test_password'
    proxy = G(models.Proxy)
    old_password_hash = proxy.password
    resp = client.patch(
        reverse('api:proxy', kwargs={'pk': proxy.id}),
        data={
            'password': PASSWORD,
        },
        content_type="application/json",
        headers=authorization_header
    )
    assert resp.status_code == status.HTTP_200_OK
    proxy.refresh_from_db()
    assert old_password_hash != proxy.password


@pytest.mark.django_db(transaction=True)
def test_remove_proxy(client, default_user, authorization_header):
    proxy = G(models.Proxy)
    resp = client.delete(
        reverse('api:proxy', kwargs={'pk': proxy.id}),
        headers=authorization_header
    )
    assert resp.status_code == status.HTTP_204_NO_CONTENT
    assert models.Proxy.objects.filter(id=proxy.id).count() == 0
