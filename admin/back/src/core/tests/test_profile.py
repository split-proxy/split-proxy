import pytest

from django.urls import reverse
from rest_framework import status
from django.contrib.auth import get_user_model
from ddf import G

User = get_user_model()


@pytest.mark.django_db(transaction=True)
def test_current_profiles(client, default_user, authorization_header):
    resp = client.get(reverse('api:current_profile'), headers=authorization_header)
    assert resp.status_code == status.HTTP_200_OK
    assert resp.json()['username'] == default_user.username


@pytest.mark.django_db(transaction=True)
def test_get_profiles(client, authorization_header):
    resp = client.get(reverse('api:profile'), headers=authorization_header)
    assert resp.status_code == status.HTTP_200_OK
    assert resp.json()['count'] == User.objects.count()


@pytest.mark.django_db(transaction=True)
def test_create_profile(client, authorization_header):
    USERNAME = 'test_user'
    resp = client.post(
        reverse('api:profile'),
        data={
            'username': USERNAME,
            'password': 'test_password',
        },
        headers=authorization_header
    )
    assert resp.status_code == status.HTTP_201_CREATED
    assert User.objects.filter(username=USERNAME).count() == 1


@pytest.mark.django_db(transaction=True)
def test_update_profile(client, default_user, authorization_header):
    PASSWORD = 'test_password'
    old_password_hash = default_user.password
    resp = client.patch(
        reverse('api:profile', kwargs={'pk': default_user.id}),
        data={
            'password': PASSWORD,
        },
        content_type="application/json",
        headers=authorization_header
    )
    assert resp.status_code == status.HTTP_200_OK
    default_user.refresh_from_db()
    assert old_password_hash != default_user.password

    data = {
        'username': default_user.username,
        'password': PASSWORD
    }
    resp = client.post(reverse('api:get_token'), data=data, content_type="application/json")
    assert resp.status_code == status.HTTP_200_OK
    assert 'token' in resp.json()


@pytest.mark.django_db(transaction=True)
def test_remove_profile(client, default_user, authorization_header):
    profile = G(User)
    resp = client.delete(
        reverse('api:profile', kwargs={'pk': profile.id}),
        headers=authorization_header
    )
    assert resp.status_code == status.HTTP_204_NO_CONTENT
    assert User.objects.filter(id=profile.id).count() == 0

    profile = G(User)
    resp = client.delete(
        reverse('api:profile', kwargs={'pk': default_user.id}),
        headers=authorization_header
    )
    assert resp.status_code == status.HTTP_403_FORBIDDEN
