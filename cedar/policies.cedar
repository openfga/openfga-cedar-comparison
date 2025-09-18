// Document Management Authorization Policies

// Organization member can view organization documents
permit(
    principal,
    action == DocumentManagement::Action::"ViewDocument",
    resource
) when {
    principal.organization == resource.organization
};

// Organization member can view organization folders
permit(
    principal,
    action == DocumentManagement::Action::"ViewFolder",
    resource
) when {
    principal.organization == resource.organization
};

// Document owner can perform all actions on their documents
permit(
    principal,
    action in [
        DocumentManagement::Action::"ViewDocument",
        DocumentManagement::Action::"EditDocument",
        DocumentManagement::Action::"DeleteDocument",
        DocumentManagement::Action::"ShareDocument"
    ],
    resource
) when {
    principal == resource.owner
};

// Document editor can edit and share documents they edit
permit(
    principal,
    action in [
        DocumentManagement::Action::"EditDocument",
        DocumentManagement::Action::"ShareDocument"
    ],
    resource
) when {
    principal in resource.editors
};

// Document viewer can view documents
permit(
    principal,
    action == DocumentManagement::Action::"ViewDocument",
    resource
) when {
    principal in resource.viewers
};

// Folder owner can perform all actions on their folders
permit(
    principal,
    action in [
        DocumentManagement::Action::"ViewFolder",
        DocumentManagement::Action::"EditFolder",
        DocumentManagement::Action::"DeleteFolder",
        DocumentManagement::Action::"ShareFolder"
    ],
    resource
) when {
    principal == resource.owner
};

// Folder editor can view, edit, and share folders (but not delete)
permit(
    principal,
    action in [
        DocumentManagement::Action::"ViewFolder",
        DocumentManagement::Action::"EditFolder",
        DocumentManagement::Action::"ShareFolder"
    ],
    resource
) when {
    principal in resource.editors
};

// Folder editor can view, edit, and share documents in their folders
permit(
    principal,
    action in [
        DocumentManagement::Action::"ViewDocument",
        DocumentManagement::Action::"EditDocument",
        DocumentManagement::Action::"ShareDocument"
    ],
    resource
) when {
    principal in resource.parent_folder.editors
};

// Folder owner can view, edit, and share documents in their folders
permit(
    principal,
    action in [
        DocumentManagement::Action::"ViewDocument",
        DocumentManagement::Action::"EditDocument",
        DocumentManagement::Action::"ShareDocument"
    ],
    resource
) when {
    principal == resource.parent_folder.owner
};

// Folder viewers can view folders
permit(
    principal,
    action == DocumentManagement::Action::"ViewFolder",
    resource
) when {
    principal in resource.viewers
};

// Folder viewers can view documents in folders
permit(
    principal,
    action == DocumentManagement::Action::"ViewDocument",
    resource
) when {
    principal in resource.parent_folder.viewers
};
