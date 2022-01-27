Feature: Test the behaviors of *sql.DB

    Scenario: Test database setup without any interactions from the application
        Given there are no rows in table "customer" of database "default"
        And these rows are stored in table "customer" of database "default"
            | id | country | first_name | last_name | email                | created_at           | updated_at |
            | 1  | US      | John       | Doe       | john.doe@example.com | 2022-02-22T20:00:02Z | NULL       |

        Then only these rows are available in table "customer" of database "default"
            | id | country | first_name | last_name | email                | created_at           | updated_at |
            | 1  | US      | John       | Doe       | john.doe@example.com | 2022-02-22T20:00:02Z | NULL       |

    Scenario Outline: Create a new customer in an empty database
        Given I <mode> database preparer
        And there are no rows in table "customer" of database "default"

        When I create a new customer:
            """
            {
                "id": 2,
                "country": "US",
                "first_name": "Jane",
                "last_name": "Doe",
                "email": "jane.doe@example.com",
                "created_at": "2022-02-22T20:00:02Z"
            }
            """

        Then I got no error
        And only these rows are available in table "customer" of database "default"
            | id | country | first_name | last_name | email                | created_at           | updated_at |
            | 2  | US      | Jane       | Doe       | jane.doe@example.com | 2022-02-22T20:00:02Z | NULL       |

        Examples:
            | mode       |
            | use        |
            | do not use |

    Scenario Outline: Create a new customer but email exists
        Given I <mode> database preparer
        And there are no rows in table "customer" of database "default"
        And these rows are stored in table "customer" of database "default"
            | id | country | first_name | last_name | email                | created_at           | updated_at |
            | 1  | US      | John       | Doe       | john.doe@example.com | 2022-02-22T20:00:02Z | NULL       |

        When I create a new customer:
            """
            {
                "id": 2,
                "country": "US",
                "first_name": "Should",
                "last_name": "Conflict",
                "email": "john.doe@example.com",
                "created_at": "2022-02-22T20:00:02Z"
            }
            """

        Then I got an error
        And only these rows are available in table "customer" of database "default"
            | id | country | first_name | last_name | email                | created_at           | updated_at |
            | 1  | US      | John       | Doe       | john.doe@example.com | 2022-02-22T20:00:02Z | NULL       |

        Examples:
            | mode       |
            | use        |
            | do not use |

    Scenario Outline: Update a customer
        Given now is "2022-12-22T20:00:02Z"
        And I <mode> database preparer
        And there are no rows in table "customer" of database "default"
        And these rows are stored in table "customer" of database "default"
            | id | country | first_name | last_name | email                | created_at           | updated_at |
            | 1  | US      | John       | Doe       | john.doe@example.com | 2022-02-22T20:00:02Z | NULL       |
            | 2  | US      | Jane       | Doe       | jane.doe@example.com | 2022-02-22T20:00:02Z | NULL       |

        When I update a customer by id "2":
            """
            {
                "country": "US",
                "first_name": "Modified",
                "last_name": "Modified",
                "email": "jane.doe@example.com",
                "created_at": "2022-02-22T20:00:02Z"
            }
            """

        Then I got no error
        And only these rows are available in table "customer" of database "default"
            | id | country | first_name | last_name | email                | created_at           | updated_at           |
            | 1  | US      | John       | Doe       | john.doe@example.com | 2022-02-22T20:00:02Z | NULL                 |
            | 2  | US      | Modified   | Modified  | jane.doe@example.com | 2022-02-22T20:00:02Z | 2022-12-22T20:00:02Z |

        When I update a customer by id "2":
            """
            {
                "email": "john.doe@example.com"
            }
            """

        Then I got an error
        And only these rows are available in table "customer" of database "default"
            | id | country | first_name | last_name | email                | created_at           | updated_at           |
            | 1  | US      | John       | Doe       | john.doe@example.com | 2022-02-22T20:00:02Z | NULL                 |
            | 2  | US      | Modified   | Modified  | jane.doe@example.com | 2022-02-22T20:00:02Z | 2022-12-22T20:00:02Z |

        Examples:
            | mode       |
            | use        |
            | do not use |

    Scenario Outline: Could not find a customer by id
        Given I <mode> database preparer
        And there are no rows in table "customer" of database "default"

        When I find a customer by id "3"

        Then I got an error

        Examples:
            | mode       |
            | use        |
            | do not use |

    Scenario Outline: Find a customer by id
        Given I <mode> database preparer
        And there are no rows in table "customer" of database "default"
        And these rows are stored in table "customer" of database "default"
            | id | country | first_name | last_name | email                  | created_at           | updated_at |
            | 3  | US      | John       | Smith     | john.smith@example.com | 2022-02-22T20:00:02Z | NULL       |

        When I find a customer by id "3"

        Then I got no error
        And I got a customer:
            """
            {
                "id": 3,
                "country": "US",
                "first_name": "John",
                "last_name": "Smith",
                "email": "john.smith@example.com",
                "created_at": "2022-02-22T20:00:02Z",
                "updated_at": "<ignore-diff>"
            }
            """

        Examples:
            | mode       | ignore-diff   |
            | use        | <ignore-diff> |
            | do not use | <ignore-diff> |

    Scenario Outline: Find all customers
        Given I <mode> database preparer
        And there are no rows in table "customer" of database "default"
        And these rows are stored in table "customer" of database "default"
            | id | country | first_name | last_name | email                | created_at           | updated_at |
            | 1  | US      | John       | Doe       | john.doe@example.com | 2022-02-22T20:00:02Z | NULL       |
            | 2  | US      | Jane       | Doe       | jane.doe@example.com | 2022-02-22T20:00:02Z | NULL       |

        When I find all customers

        Then I got no error
        And I got these customers:
            """
            [
                {
                    "id": 1,
                    "country": "US",
                    "first_name": "John",
                    "last_name": "Doe",
                    "email": "john.doe@example.com",
                    "created_at": "2022-02-22T20:00:02Z",
                    "updated_at": "<ignore-diff>"
                },
                {
                    "id": 2,
                    "country": "US",
                    "first_name": "Jane",
                    "last_name": "Doe",
                    "email": "jane.doe@example.com",
                    "created_at": "2022-02-22T20:00:02Z",
                    "updated_at": "<ignore-diff>"
                }
            ]
            """

        Examples:
            | mode       | ignore-diff   |
            | use        | <ignore-diff> |
            | do not use | <ignore-diff> |

    Scenario Outline: Delete a customer by id
        Given I <mode> database preparer
        And there are no rows in table "customer" of database "default"
        And these rows are stored in table "customer" of database "default"
            | id | country | first_name | last_name | email                  | created_at           | updated_at |
            | 3  | US      | John       | Smith     | john.smith@example.com | 2022-02-22T20:00:02Z | NULL       |

        When I delete a customer by id "3"

        Then I got no error
        And no rows are available in table "customer" of database "default"

        Examples:
            | mode       |
            | use        |
            | do not use |
