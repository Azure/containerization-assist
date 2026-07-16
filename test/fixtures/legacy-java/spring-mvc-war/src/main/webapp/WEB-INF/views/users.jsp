<%@ taglib prefix="c" uri="http://java.sun.com/jsp/jstl/core" %>
<%@ page contentType="text/html;charset=UTF-8" language="java" %>
<html>
<head><title>Users</title></head>
<body>
    <h1>Users</h1>
    <table border="1" cellpadding="6">
        <tr><th>ID</th><th>Username</th><th>Email</th></tr>
        <c:forEach var="u" items="${users}">
            <tr>
                <td><c:out value="${u.id}" /></td>
                <td><c:out value="${u.username}" /></td>
                <td><c:out value="${u.email}" /></td>
            </tr>
        </c:forEach>
    </table>
</body>
</html>
