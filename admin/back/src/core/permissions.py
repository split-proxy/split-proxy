from rest_framework.permissions import BasePermission


class CanDeleteProfile(BasePermission):
    message = "Superuser profile cannot be deleted."

    def has_object_permission(self, request, view, obj):
        if request.method == "DELETE" and obj.is_superuser:
            return False

        return True
